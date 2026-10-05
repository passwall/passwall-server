package gormrepo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/pkg/database"
	"gorm.io/gorm"
)

// AdminDirectoryStatsRepository reads account usage counts for the admin directory.
// Queries select counts and timestamps only. They do not select vault payloads.
type AdminDirectoryStatsRepository struct {
	db *gorm.DB
}

// NewAdminDirectoryStatsRepository creates the usage reader.
func NewAdminDirectoryStatsRepository(db *gorm.DB) *AdminDirectoryStatsRepository {
	return &AdminDirectoryStatsRepository{db: db}
}

type directorySchemaRef struct {
	userID uint
	schema string
}

type directoryTypeCount struct {
	UserID    uint  `gorm:"column:user_id"`
	ItemType  int16 `gorm:"column:item_type"`
	ItemCount int64 `gorm:"column:item_count"`
}

type directoryOrgCount struct {
	UserID uint  `gorm:"column:user_id"`
	Count  int64 `gorm:"column:count"`
}

type directoryActivityCount struct {
	UserID         uint         `gorm:"column:user_id"`
	ActivityCount  int64        `gorm:"column:activity_count"`
	SignInCount    int64        `gorm:"column:signin_count"`
	LastActivityAt sql.NullTime `gorm:"column:last_activity_at"`
}

type directoryClientCount struct {
	UserID      uint  `gorm:"column:user_id"`
	DeviceCount int64 `gorm:"column:device_count"`
	ClientCount int64 `gorm:"column:client_count"`
}

// ForUsers returns usage for each non-nil user. A nil Vault means the personal
// schema was missing, blank, public, or had no items table. Other counters are
// zero when the shared tables have no matching rows.
func (r *AdminDirectoryStatsRepository) ForUsers(ctx context.Context, users []*domain.User) (map[uint]domain.AdminDirectoryUsage, error) {
	out := make(map[uint]domain.AdminDirectoryUsage)
	if r == nil || r.db == nil {
		return nil, errors.New("admin directory stats repository is not configured")
	}

	ids := make([]uint, 0, len(users))
	schemas := make([]directorySchemaRef, 0, len(users))
	seen := make(map[uint]struct{}, len(users))
	for _, user := range users {
		if user == nil {
			continue
		}
		if _, ok := seen[user.ID]; ok {
			continue
		}
		seen[user.ID] = struct{}{}
		ids = append(ids, user.ID)
		out[user.ID] = domain.AdminDirectoryUsage{}
		schemas = append(schemas, directorySchemaRef{userID: user.ID, schema: user.Schema})
	}
	if len(ids) == 0 {
		return out, nil
	}

	vaults, err := r.personalVaultCounts(ctx, schemas)
	if err != nil {
		return nil, err
	}
	orgCounts, err := r.organizationCounts(ctx, ids)
	if err != nil {
		return nil, err
	}
	collectionCounts, err := r.collectionCounts(ctx, ids)
	if err != nil {
		return nil, err
	}
	orgItems, err := r.organizationItemCounts(ctx, ids)
	if err != nil {
		return nil, err
	}
	activities, err := r.activityCounts(ctx, ids)
	if err != nil {
		return nil, err
	}
	clients, err := r.clientCounts(ctx, ids)
	if err != nil {
		return nil, err
	}

	for _, id := range ids {
		usage := out[id]
		if vault, known := vaults[id]; known {
			usage.Vault = vault
		}
		usage.OrganizationCount = orgCounts[id]
		usage.CollectionCount = collectionCounts[id]
		if items, ok := orgItems[id]; ok {
			usage.OrganizationItems = items
		}
		if activity, ok := activities[id]; ok {
			usage.ActivityCount = activity.ActivityCount
			usage.SignInCount = activity.SignInCount
			usage.LastActivityAt = activity.LastActivityAt
		}
		if client, ok := clients[id]; ok {
			usage.DeviceCount = client.DeviceCount
			usage.ClientCount = client.ClientCount
		}
		out[id] = usage
	}
	return out, nil
}

func (r *AdminDirectoryStatsRepository) personalVaultCounts(ctx context.Context, refs []directorySchemaRef) (map[uint]*domain.AdminDirectoryItemCounts, error) {
	out := make(map[uint]*domain.AdminDirectoryItemCounts, len(refs))
	countable := make([]directorySchemaRef, 0, len(refs))
	names := make([]string, 0, len(refs))
	seenSchema := make(map[string]struct{}, len(refs))
	for _, ref := range refs {
		if !directorySchemaCountable(ref.schema) {
			continue
		}
		countable = append(countable, ref)
		if _, ok := seenSchema[ref.schema]; ok {
			continue
		}
		seenSchema[ref.schema] = struct{}{}
		names = append(names, ref.schema)
	}
	if len(countable) == 0 {
		return out, nil
	}

	existing, err := r.existingSchemas(ctx, names)
	if err != nil {
		return nil, err
	}
	present := make([]directorySchemaRef, 0, len(countable))
	for _, ref := range countable {
		if existing[ref.schema] {
			present = append(present, ref)
		}
	}
	if len(present) == 0 {
		return out, nil
	}

	counted, err := r.countPersonalSchemas(ctx, present)
	if err != nil {
		if !isMissingRelation(err) {
			return nil, err
		}
		counted, err = r.countPersonalSchemasOneByOne(ctx, present)
		if err != nil {
			return nil, err
		}
	}
	for userID, counts := range counted {
		copied := counts
		out[userID] = &copied
	}
	return out, nil
}

func (r *AdminDirectoryStatsRepository) existingSchemas(ctx context.Context, names []string) (map[string]bool, error) {
	type schemaNameRow struct {
		Name string `gorm:"column:nspname"`
	}
	var found []schemaNameRow
	err := r.db.WithContext(ctx).
		Raw("SELECT nspname FROM pg_namespace WHERE nspname IN ?", names).
		Scan(&found).Error
	if err != nil {
		return nil, err
	}
	out := make(map[string]bool, len(found))
	for _, name := range found {
		out[name.Name] = true
	}
	return out, nil
}

func (r *AdminDirectoryStatsRepository) countPersonalSchemas(ctx context.Context, refs []directorySchemaRef) (map[uint]domain.AdminDirectoryItemCounts, error) {
	parts := make([]string, 0, len(refs))
	included := make([]directorySchemaRef, 0, len(refs))
	for _, ref := range refs {
		table, err := database.BuildQualifiedTableName(ref.schema, "items")
		if err != nil {
			continue
		}
		parts = append(parts, fmt.Sprintf(
			"SELECT %d AS user_id, item_type, COUNT(*) AS item_count FROM %s WHERE deleted_at IS NULL GROUP BY item_type",
			ref.userID,
			table,
		))
		included = append(included, ref)
	}
	out := make(map[uint]domain.AdminDirectoryItemCounts, len(included))
	for _, ref := range included {
		out[ref.userID] = domain.AdminDirectoryItemCounts{}
	}
	if len(parts) == 0 {
		return out, nil
	}

	var rows []directoryTypeCount
	err := r.db.WithContext(ctx).Raw(strings.Join(parts, " UNION ALL ")).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		counts := out[row.UserID]
		counts.Add(domain.ItemType(row.ItemType), int(row.ItemCount))
		out[row.UserID] = counts
	}
	return out, nil
}

func (r *AdminDirectoryStatsRepository) countPersonalSchemasOneByOne(ctx context.Context, refs []directorySchemaRef) (map[uint]domain.AdminDirectoryItemCounts, error) {
	out := make(map[uint]domain.AdminDirectoryItemCounts, len(refs))
	for _, ref := range refs {
		table, err := database.BuildQualifiedTableName(ref.schema, "items")
		if err != nil {
			continue
		}
		var rows []directoryTypeCount
		err = r.db.WithContext(ctx).Raw(
			fmt.Sprintf("SELECT item_type, COUNT(*) AS item_count FROM %s WHERE deleted_at IS NULL GROUP BY item_type", table),
		).Scan(&rows).Error
		if err != nil {
			if isMissingRelation(err) {
				continue
			}
			return nil, err
		}
		counts := domain.AdminDirectoryItemCounts{}
		for _, row := range rows {
			counts.Add(domain.ItemType(row.ItemType), int(row.ItemCount))
		}
		out[ref.userID] = counts
	}
	return out, nil
}

func (r *AdminDirectoryStatsRepository) organizationCounts(ctx context.Context, userIDs []uint) (map[uint]int, error) {
	var rows []directoryOrgCount
	err := r.db.WithContext(ctx).Raw(`
		SELECT user_id, COUNT(*) AS count
		FROM organization_users
		WHERE user_id IN ?
		GROUP BY user_id
	`, userIDs).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	return orgCountMap(rows), nil
}

func (r *AdminDirectoryStatsRepository) collectionCounts(ctx context.Context, userIDs []uint) (map[uint]int, error) {
	var rows []directoryOrgCount
	err := r.db.WithContext(ctx).Raw(`
		SELECT ou.user_id AS user_id, COUNT(DISTINCT c.id) AS count
		FROM organization_users ou
		INNER JOIN collections c
			ON c.organization_id = ou.organization_id
			AND c.deleted_at IS NULL
		WHERE ou.user_id IN ?
		GROUP BY ou.user_id
	`, userIDs).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	return orgCountMap(rows), nil
}

func (r *AdminDirectoryStatsRepository) organizationItemCounts(ctx context.Context, userIDs []uint) (map[uint]domain.AdminDirectoryItemCounts, error) {
	var rows []directoryTypeCount
	err := r.db.WithContext(ctx).Raw(`
		SELECT created_by_user_id AS user_id, item_type, COUNT(*) AS item_count
		FROM organization_items
		WHERE deleted_at IS NULL AND created_by_user_id IN ?
		GROUP BY created_by_user_id, item_type
	`, userIDs).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make(map[uint]domain.AdminDirectoryItemCounts)
	for _, row := range rows {
		counts := out[row.UserID]
		counts.Add(domain.ItemType(row.ItemType), int(row.ItemCount))
		out[row.UserID] = counts
	}
	return out, nil
}

func (r *AdminDirectoryStatsRepository) activityCounts(ctx context.Context, userIDs []uint) (map[uint]directoryActivityUsage, error) {
	var rows []directoryActivityCount
	err := r.db.WithContext(ctx).Raw(`
		SELECT user_id,
			COUNT(*) AS activity_count,
			COUNT(*) FILTER (WHERE activity_type = ?) AS signin_count,
			MAX(created_at) FILTER (WHERE activity_type <> ?) AS last_activity_at
		FROM user_activities
		WHERE user_id IN ?
		GROUP BY user_id
	`, domain.ActivityTypeSignIn, domain.ActivityTypeSignIn, userIDs).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make(map[uint]directoryActivityUsage, len(rows))
	for _, row := range rows {
		usage := directoryActivityUsage{
			ActivityCount: int(row.ActivityCount),
			SignInCount:   int(row.SignInCount),
		}
		if row.LastActivityAt.Valid && !row.LastActivityAt.Time.IsZero() {
			activityAt := row.LastActivityAt.Time
			usage.LastActivityAt = &activityAt
		}
		out[row.UserID] = usage
	}
	return out, nil
}

type directoryActivityUsage struct {
	ActivityCount  int
	SignInCount    int
	LastActivityAt *time.Time
}

type directoryClientUsage struct {
	DeviceCount int
	ClientCount int
}

func (r *AdminDirectoryStatsRepository) clientCounts(ctx context.Context, userIDs []uint) (map[uint]directoryClientUsage, error) {
	var rows []directoryClientCount
	err := r.db.WithContext(ctx).Raw(`
		SELECT user_id,
			COUNT(DISTINCT device_id) FILTER (
				WHERE device_id IS NOT NULL
					AND device_id <> '00000000-0000-0000-0000-000000000000'::uuid
			) AS device_count,
			COUNT(DISTINCT app) FILTER (
				WHERE app IS NOT NULL AND app <> ''
			) AS client_count
		FROM tokens
		WHERE user_id IN ?
		GROUP BY user_id
	`, userIDs).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make(map[uint]directoryClientUsage, len(rows))
	for _, row := range rows {
		out[row.UserID] = directoryClientUsage{
			DeviceCount: int(row.DeviceCount),
			ClientCount: int(row.ClientCount),
		}
	}
	return out, nil
}

func orgCountMap(rows []directoryOrgCount) map[uint]int {
	out := make(map[uint]int, len(rows))
	for _, row := range rows {
		out[row.UserID] = int(row.Count)
	}
	return out
}

func directorySchemaCountable(schema string) bool {
	if schema == "" || schema == "public" {
		return false
	}
	return database.ValidateSchemaName(schema) == nil
}

func isMissingRelation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		// 42P01 undefined_table, 3F000 invalid_schema_name.
		return pgErr.Code == "42P01" || pgErr.Code == "3F000"
	}
	return false
}
