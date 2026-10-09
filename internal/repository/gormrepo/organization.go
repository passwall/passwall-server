package gormrepo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type organizationRepository struct {
	db *gorm.DB
}

// NewOrganizationRepository creates a new organization repository
func NewOrganizationRepository(db *gorm.DB) repository.OrganizationRepository {
	return &organizationRepository{db: db}
}

func (r *organizationRepository) Create(ctx context.Context, org *domain.Organization) error {
	if org.UUID == uuid.Nil {
		org.UUID = uuid.New()
	}

	if org.PublicID == "" {
		pid, err := domain.GeneratePublicID()
		if err != nil {
			return fmt.Errorf("generate public_id: %w", err)
		}
		org.PublicID = pid
	}

	return dbFromContext(ctx, r.db).Create(org).Error
}

func (r *organizationRepository) GetByID(ctx context.Context, id uint) (*domain.Organization, error) {
	var org domain.Organization
	err := dbFromContext(ctx, r.db).Where("id = ?", id).First(&org).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &org, nil
}

func (r *organizationRepository) GetByUUID(ctx context.Context, uuidStr string) (*domain.Organization, error) {
	var org domain.Organization
	err := dbFromContext(ctx, r.db).Where("uuid = ?", uuidStr).First(&org).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &org, nil
}

func (r *organizationRepository) GetByPublicID(ctx context.Context, publicID string) (*domain.Organization, error) {
	var org domain.Organization
	err := dbFromContext(ctx, r.db).Where("public_id = ?", publicID).First(&org).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &org, nil
}

func (r *organizationRepository) List(ctx context.Context, filter repository.ListFilter) ([]*domain.Organization, *repository.ListResult, error) {
	var orgs []*domain.Organization
	var total int64

	query := dbFromContext(ctx, r.db).Model(&domain.Organization{})

	// Count total
	if err := query.Count(&total).Error; err != nil {
		return nil, nil, err
	}

	// Apply search filter (case-insensitive)
	if filter.Search != "" {
		searchPattern := "%" + filter.Search + "%"
		if filter.SearchOwners {
			query = query.Where(
				`LOWER(organizations.name) LIKE LOWER(?)
				 OR LOWER(COALESCE(organizations.billing_email, '')) LIKE LOWER(?)
				 OR EXISTS (
					SELECT 1 FROM organization_users
					JOIN users ON users.id = organization_users.user_id
					WHERE organization_users.organization_id = organizations.id
					  AND organization_users.role = ?
					  AND (LOWER(users.email) LIKE LOWER(?) OR LOWER(COALESCE(users.name, '')) LIKE LOWER(?))
				 )`,
				searchPattern, searchPattern, domain.OrgRoleOwner, searchPattern, searchPattern,
			)
		} else {
			query = query.Where(
				"LOWER(name) LIKE LOWER(?) OR LOWER(COALESCE(billing_email, '')) LIKE LOWER(?)",
				searchPattern, searchPattern,
			)
		}
	}
	if filter.OwnerUserID > 0 {
		query = query.Where(
			`organizations.personal_owner_user_id = ?
			 OR organizations.created_by_user_id = ?
			 OR EXISTS (
				SELECT 1 FROM organization_users
				WHERE organization_users.organization_id = organizations.id
				  AND organization_users.user_id = ?
				  AND organization_users.role = ?
			 )`,
			filter.OwnerUserID,
			filter.OwnerUserID,
			filter.OwnerUserID,
			domain.OrgRoleOwner,
		)
	}

	if filter.ManualGrantEndsBefore != nil {
		query = query.Where(
			`EXISTS (
				SELECT 1 FROM subscriptions
				JOIN plans ON plans.id = subscriptions.plan_id
				WHERE subscriptions.organization_id = organizations.id
				  AND subscriptions.state = ?
				  AND (subscriptions.stripe_subscription_id IS NULL OR TRIM(subscriptions.stripe_subscription_id) = '')
				  AND subscriptions.renew_at > ?
				  AND subscriptions.renew_at <= ?
				  AND plans.price_cents > 0
			)`,
			domain.SubStateActive, time.Now(), *filter.ManualGrantEndsBefore,
		)
	}

	// Count filtered
	var filtered int64
	if err := query.Count(&filtered).Error; err != nil {
		return nil, nil, err
	}

	// Apply pagination
	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
	}
	if filter.Offset > 0 {
		query = query.Offset(filter.Offset)
	}

	// Apply sorting
	if filter.SortByAccessEndAsc {
		// Upcoming end of the current access period (manual end date, renewal
		// or cancel date); organizations without one sort last. A single
		// expression is required: GORM drops an OrderBy expression when a
		// column order is merged after it.
		query = query.Order(clause.OrderBy{Expression: clause.Expr{
			SQL: `(SELECT MIN(subscriptions.renew_at) FROM subscriptions
				WHERE subscriptions.organization_id = organizations.id
				  AND subscriptions.state IN ('` + string(domain.SubStateActive) + `', '` + string(domain.SubStateTrialing) + `', '` +
				string(domain.SubStatePastDue) + `', '` + string(domain.SubStateCanceled) + `')
				  AND subscriptions.renew_at > ?) ASC NULLS LAST, organizations.created_at DESC`,
			Vars:               []interface{}{time.Now()},
			WithoutParentheses: true,
		}})
	} else {
		orderBy := "created_at DESC"
		if filter.Sort != "" {
			order := "ASC"
			if filter.Order == "desc" {
				order = "DESC"
			}
			orderBy = fmt.Sprintf("%s %s", filter.Sort, order)
		}
		query = query.Order(orderBy)
	}

	// Execute query
	if err := query.Find(&orgs).Error; err != nil {
		return nil, nil, err
	}

	result := &repository.ListResult{
		Total:    total,
		Filtered: filtered,
	}

	return orgs, result, nil
}

// GetDefaultByOwnerID returns the user's default (personal) organization where they are the owner.
// Every user gets a default organization when they sign up (IsDefault=true).
func (r *organizationRepository) GetDefaultByOwnerID(ctx context.Context, ownerUserID uint) (*domain.Organization, error) {
	var org domain.Organization

	// Default org is defined by organizations.is_default=true.
	// Do NOT couple this to organization_users.role or organizations.created_by_user_id; both can be
	// missing/legacy in migrated data.
	err := dbFromContext(ctx, r.db).
		Joins("JOIN organization_users ON organization_users.organization_id = organizations.id").
		Where("organization_users.user_id = ?", ownerUserID).
		Where("organization_users.status IN ?", []domain.OrganizationUserStatus{
			domain.OrgUserStatusAccepted,
			domain.OrgUserStatusConfirmed,
		}).
		Where("organizations.is_default = ?", true).
		Where("organizations.is_active = ?", true).
		First(&org).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}

	return &org, nil
}

func (r *organizationRepository) ListForUser(ctx context.Context, userID uint) ([]*domain.Organization, error) {
	var orgs []*domain.Organization

	err := dbFromContext(ctx, r.db).
		Joins("JOIN organization_users ON organization_users.organization_id = organizations.id").
		Where("organization_users.user_id = ? AND organization_users.status IN ?", userID, []domain.OrganizationUserStatus{
			domain.OrgUserStatusAccepted,
			domain.OrgUserStatusConfirmed,
		}).
		Where("organizations.is_active = ?", true).
		Order("organizations.name ASC").
		Find(&orgs).Error

	if err != nil {
		return nil, err
	}

	if len(orgs) == 0 {
		return orgs, nil
	}

	// Batch-fetch user-specific wrapped org keys from organization_users.
	// Each member has their own encrypted_org_key (org key wrapped with their User Key).
	// The organizations table stores the owner's copy, which is wrong for non-owner members.
	orgIDs := make([]uint, len(orgs))
	for i, o := range orgs {
		orgIDs[i] = o.ID
	}

	type orgKeyRow struct {
		OrganizationID  uint   `gorm:"column:organization_id"`
		EncryptedOrgKey string `gorm:"column:encrypted_org_key"`
	}
	var keys []orgKeyRow
	if err := dbFromContext(ctx, r.db).
		Table("organization_users").
		Select("organization_id, encrypted_org_key").
		Where("user_id = ? AND organization_id IN ?", userID, orgIDs).
		Find(&keys).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch user org keys: %w", err)
	}

	keyByOrg := make(map[uint]string, len(keys))
	for _, k := range keys {
		keyByOrg[k.OrganizationID] = k.EncryptedOrgKey
	}
	for _, org := range orgs {
		if key, ok := keyByOrg[org.ID]; ok {
			org.EncryptedOrgKey = key
		}
	}

	return orgs, nil
}

func (r *organizationRepository) Update(ctx context.Context, org *domain.Organization) error {
	return dbFromContext(ctx, r.db).Save(org).Error
}

func (r *organizationRepository) Delete(ctx context.Context, id uint) error {
	// Personal Vault organizations are never deletable (by any flow).
	var org domain.Organization
	if err := dbFromContext(ctx, r.db).
		Select("id", "is_personal").
		Where("id = ?", id).
		First(&org).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return repository.ErrNotFound
		}
		return err
	}
	if org.IsPersonal {
		return repository.ErrForbidden
	}

	// Purge organization data, but DO NOT delete organization row.
	// We keep the org record to preserve billing/subscription history and auditability.
	return dbFromContext(ctx, r.db).Transaction(func(tx *gorm.DB) error {
		// Items first (may reference collections)
		if err := tx.Unscoped().
			Where("organization_id = ?", id).
			Delete(&domain.OrganizationItem{}).Error; err != nil {
			return err
		}

		// Invitations that reference the org (cleanup)
		if err := tx.Where("organization_id = ?", id).
			Delete(&domain.Invitation{}).Error; err != nil {
			return err
		}

		// Collection access tables (be robust even if FKs aren't cascading in older schemas)
		if err := tx.Exec(`
DELETE FROM collection_users
WHERE collection_id IN (SELECT id FROM collections WHERE organization_id = ?)
`, id).Error; err != nil {
			return err
		}
		if err := tx.Exec(`
DELETE FROM collection_teams
WHERE collection_id IN (SELECT id FROM collections WHERE organization_id = ?)
   OR team_id IN (SELECT id FROM teams WHERE organization_id = ?)
`, id, id).Error; err != nil {
			return err
		}

		// Team memberships (robust cleanup)
		if err := tx.Exec(`
DELETE FROM team_users
WHERE team_id IN (SELECT id FROM teams WHERE organization_id = ?)
   OR organization_user_id IN (SELECT id FROM organization_users WHERE organization_id = ?)
`, id, id).Error; err != nil {
			return err
		}

		// Delete teams & collections (hard)
		if err := tx.Unscoped().
			Where("organization_id = ?", id).
			Delete(&domain.Team{}).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().
			Where("organization_id = ?", id).
			Delete(&domain.Collection{}).Error; err != nil {
			return err
		}

		// Remove org memberships (hard)
		if err := tx.Unscoped().
			Where("organization_id = ?", id).
			Delete(&domain.OrganizationUser{}).Error; err != nil {
			return err
		}

		// Mark organization as deleted/inactive (keep row)
		now := time.Now()
		return tx.Model(&domain.Organization{}).
			Where("id = ?", id).
			Updates(map[string]any{
				"status":     domain.OrgStatusDeleted,
				"is_active":  false,
				"deleted_at": &now,
			}).Error
	})
}

func (r *organizationRepository) PurgePersonal(ctx context.Context, id uint) error {
	var org domain.Organization
	if err := dbFromContext(ctx, r.db).
		Select("id", "is_personal").
		Where("id = ?", id).
		First(&org).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return repository.ErrNotFound
		}
		return err
	}
	if !org.IsPersonal {
		return repository.ErrForbidden
	}

	return dbFromContext(ctx, r.db).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("organization_id = ?", id).Delete(&domain.ItemShare{}).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Where("organization_id = ?", id).Delete(&domain.OrganizationItem{}).Error; err != nil {
			return err
		}
		if err := tx.Where("organization_id = ?", id).Delete(&domain.Invitation{}).Error; err != nil {
			return err
		}
		if err := tx.Exec(`
DELETE FROM collection_users
WHERE collection_id IN (SELECT id FROM collections WHERE organization_id = ?)
`, id).Error; err != nil {
			return err
		}
		if err := tx.Exec(`
DELETE FROM collection_teams
WHERE collection_id IN (SELECT id FROM collections WHERE organization_id = ?)
   OR team_id IN (SELECT id FROM teams WHERE organization_id = ?)
`, id, id).Error; err != nil {
			return err
		}
		if err := tx.Exec(`
DELETE FROM team_users
WHERE team_id IN (SELECT id FROM teams WHERE organization_id = ?)
   OR organization_user_id IN (SELECT id FROM organization_users WHERE organization_id = ?)
`, id, id).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Where("organization_id = ?", id).Delete(&domain.OrganizationFolder{}).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Where("organization_id = ?", id).Delete(&domain.Team{}).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Where("organization_id = ?", id).Delete(&domain.Collection{}).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Where("organization_id = ?", id).Delete(&domain.OrganizationUser{}).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Where("organization_id = ?", id).Delete(&domain.Subscription{}).Error; err != nil {
			return err
		}
		return tx.Unscoped().Delete(&domain.Organization{}, id).Error
	})
}

func (r *organizationRepository) GetMemberCount(ctx context.Context, orgID uint) (int, error) {
	var count int64
	err := dbFromContext(ctx, r.db).
		Model(&domain.OrganizationUser{}).
		Where("organization_id = ? AND status IN ?", orgID, []domain.OrganizationUserStatus{
			domain.OrgUserStatusAccepted,
			domain.OrgUserStatusConfirmed,
		}).
		Count(&count).Error

	return int(count), err
}

func (r *organizationRepository) GetCountsByIDs(ctx context.Context, orgIDs []uint) (map[uint]repository.OrganizationCounts, error) {
	out := make(map[uint]repository.OrganizationCounts, len(orgIDs))
	if len(orgIDs) == 0 {
		return out, nil
	}
	type row struct {
		OrganizationID uint
		Count          int
	}
	collect := func(model interface{}, where string, set func(*repository.OrganizationCounts, int), args ...interface{}) error {
		var rows []row
		err := dbFromContext(ctx, r.db).
			Model(model).
			Select("organization_id, COUNT(*) AS count").
			Where("organization_id IN ?", orgIDs).
			Where(where, args...).
			Group("organization_id").
			Scan(&rows).Error
		if err != nil {
			return err
		}
		for _, item := range rows {
			counts := out[item.OrganizationID]
			set(&counts, item.Count)
			out[item.OrganizationID] = counts
		}
		return nil
	}

	// Conditions mirror GetMemberCount, GetTeamCount, GetCollectionCount and GetItemCount.
	if err := collect(&domain.OrganizationUser{}, "status IN ?", func(c *repository.OrganizationCounts, n int) { c.Members = n },
		[]domain.OrganizationUserStatus{domain.OrgUserStatusAccepted, domain.OrgUserStatusConfirmed}); err != nil {
		return nil, err
	}
	if err := collect(&domain.Team{}, "is_default = false", func(c *repository.OrganizationCounts, n int) { c.Teams = n }); err != nil {
		return nil, err
	}
	if err := collect(&domain.Collection{}, "deleted_at IS NULL AND is_default = false", func(c *repository.OrganizationCounts, n int) { c.Collections = n }); err != nil {
		return nil, err
	}
	if err := collect(&domain.OrganizationItem{}, "deleted_at IS NULL", func(c *repository.OrganizationCounts, n int) { c.Items = n }); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *organizationRepository) GetTeamCount(ctx context.Context, orgID uint) (int, error) {
	var count int64
	err := dbFromContext(ctx, r.db).
		Model(&domain.Team{}).
		Where("organization_id = ? AND is_default = false", orgID).
		Count(&count).Error

	return int(count), err
}

func (r *organizationRepository) GetCollectionCount(ctx context.Context, orgID uint) (int, error) {
	var count int64
	err := dbFromContext(ctx, r.db).
		Model(&domain.Collection{}).
		Where("organization_id = ? AND deleted_at IS NULL AND is_default = false", orgID).
		Count(&count).Error

	return int(count), err
}

func (r *organizationRepository) GetItemCount(ctx context.Context, orgID uint) (int, error) {
	var count int64
	err := dbFromContext(ctx, r.db).
		Model(&domain.OrganizationItem{}).
		Where("organization_id = ? AND deleted_at IS NULL", orgID).
		Count(&count).Error

	return int(count), err
}
