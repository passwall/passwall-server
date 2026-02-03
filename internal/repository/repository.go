package repository

import (
	"context"
	"errors"
	"time"

	"github.com/passwall/passwall-server/internal/domain"
	uuid "github.com/satori/go.uuid"
)

// Common errors
var (
	ErrNotFound      = errors.New("record not found")
	ErrAlreadyExists = errors.New("record already exists")
	ErrUnauthorized  = errors.New("unauthorized")
	ErrInvalidInput  = errors.New("invalid input")
	ErrForbidden     = errors.New("operation forbidden")
)

// ListFilter represents common list filter parameters
type ListFilter struct {
	Search string
	Limit  int
	Offset int
	Sort   string
	Order  string
}

// ListResult represents list query results with pagination info
type ListResult struct {
	Total    int64
	Filtered int64
}

// NOTE: Legacy repository interfaces removed (Login, BankAccount, CreditCard, Note, Email, Server)
// All item types now use ItemRepository with flexible items architecture

// UserRepository defines user data access methods
type UserRepository interface {
	GetByID(ctx context.Context, id uint) (*domain.User, error)
	GetByUUID(ctx context.Context, uuid string) (*domain.User, error)
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
	GetBySchema(ctx context.Context, schema string) (*domain.User, error)
	List(ctx context.Context, filter ListFilter) ([]*domain.User, *ListResult, error)
	GetItemCount(ctx context.Context, schema string) (int, error)
	Create(ctx context.Context, user *domain.User) error
	Update(ctx context.Context, user *domain.User) error
	Delete(ctx context.Context, id uint, schema string) error
	Migrate() error
	CreateSchema(schema string) error
	MigrateUserSchema(schema string) error
}

// UserNotificationPreferencesRepository defines data access for user notification preferences.
type UserNotificationPreferencesRepository interface {
	GetByUserID(ctx context.Context, userID uint) (*domain.UserNotificationPreferences, error)
	Upsert(ctx context.Context, prefs *domain.UserNotificationPreferences) error
}

// UserAppearancePreferencesRepository defines data access for user appearance preferences.
type UserAppearancePreferencesRepository interface {
	GetByUserID(ctx context.Context, userID uint) (*domain.UserAppearancePreferences, error)
	Upsert(ctx context.Context, prefs *domain.UserAppearancePreferences) error
}

// PreferencesRepository defines data access for generic preferences (user/org scoped).
type PreferencesRepository interface {
	ListByOwner(ctx context.Context, ownerType string, ownerID uint, section string) ([]*domain.Preference, error)
	UpsertMany(ctx context.Context, prefs []*domain.Preference) error
}

// TokenRepository defines token data access methods
type TokenRepository interface {
	Create(ctx context.Context, userID int, sessionUUID uuid.UUID, deviceID uuid.UUID, app string, kind string, tokenUUID uuid.UUID, token string, expiresAt time.Time) error
	GetByUUID(ctx context.Context, uuid string) (*domain.Token, error)
	Delete(ctx context.Context, userID int) error
	DeleteByUUID(ctx context.Context, uuid string) error
	DeleteBySessionUUID(ctx context.Context, sessionUUID string) error
	DeleteExpired(ctx context.Context) (int64, error)
	Cleanup(ctx context.Context) error
	Migrate() error
}

// RoleRepository defines role data access methods
type RoleRepository interface {
	GetByID(ctx context.Context, id uint) (*domain.Role, error)
	GetByName(ctx context.Context, name string) (*domain.Role, error)
	List(ctx context.Context) ([]*domain.Role, error)
	GetPermissions(ctx context.Context, roleID uint) ([]string, error)
	Migrate() error
}

// PermissionRepository defines permission data access methods
type PermissionRepository interface {
	GetByID(ctx context.Context, id uint) (*domain.Permission, error)
	GetByName(ctx context.Context, name string) (*domain.Permission, error)
	List(ctx context.Context) ([]*domain.Permission, error)
	Migrate() error
}

// VerificationRepository defines verification code data access methods
type VerificationRepository interface {
	Create(ctx context.Context, code *domain.VerificationCode) error
	GetByEmailAndCode(ctx context.Context, email, code string) (*domain.VerificationCode, error)
	DeleteByEmail(ctx context.Context, email string) error
	DeleteExpired(ctx context.Context) (int64, error)
	Migrate() error
}

// OrganizationRepository defines organization data access methods
type OrganizationRepository interface {
	Create(ctx context.Context, org *domain.Organization) error
	GetByID(ctx context.Context, id uint) (*domain.Organization, error)
	GetByUUID(ctx context.Context, uuid string) (*domain.Organization, error)
	List(ctx context.Context, filter ListFilter) ([]*domain.Organization, *ListResult, error)
	ListForUser(ctx context.Context, userID uint) ([]*domain.Organization, error)
	Update(ctx context.Context, org *domain.Organization) error
	Delete(ctx context.Context, id uint) error

	// Stats
	GetMemberCount(ctx context.Context, orgID uint) (int, error)
	GetTeamCount(ctx context.Context, orgID uint) (int, error)
	GetCollectionCount(ctx context.Context, orgID uint) (int, error)
	GetItemCount(ctx context.Context, orgID uint) (int, error)
}

// OrganizationUserRepository defines organization user data access methods
type OrganizationUserRepository interface {
	Create(ctx context.Context, orgUser *domain.OrganizationUser) error
	GetByID(ctx context.Context, id uint) (*domain.OrganizationUser, error)
	GetByUUID(ctx context.Context, uuid string) (*domain.OrganizationUser, error)
	GetByOrgAndUser(ctx context.Context, orgID, userID uint) (*domain.OrganizationUser, error)
	ListByOrganization(ctx context.Context, orgID uint) ([]*domain.OrganizationUser, error)
	ListByUser(ctx context.Context, userID uint) ([]*domain.OrganizationUser, error)
	Update(ctx context.Context, orgUser *domain.OrganizationUser) error
	Delete(ctx context.Context, id uint) error

	// Invitations
	CountInvited(ctx context.Context, orgID uint) (int, error)
	ListPendingInvitations(ctx context.Context, userEmail string) ([]*domain.OrganizationUser, error)
}

// TeamRepository defines team data access methods
type TeamRepository interface {
	Create(ctx context.Context, team *domain.Team) error
	GetByID(ctx context.Context, id uint) (*domain.Team, error)
	GetByUUID(ctx context.Context, uuid string) (*domain.Team, error)
	GetByName(ctx context.Context, orgID uint, name string) (*domain.Team, error)
	GetDefaultByOrganization(ctx context.Context, orgID uint) (*domain.Team, error)
	ListByOrganization(ctx context.Context, orgID uint) ([]*domain.Team, error)
	Update(ctx context.Context, team *domain.Team) error
	Delete(ctx context.Context, id uint) error

	// Stats
	GetMemberCount(ctx context.Context, teamID uint) (int, error)
}

// TeamUserRepository defines team user data access methods
type TeamUserRepository interface {
	Create(ctx context.Context, teamUser *domain.TeamUser) error
	GetByID(ctx context.Context, id uint) (*domain.TeamUser, error)
	GetByTeamAndOrgUser(ctx context.Context, teamID, orgUserID uint) (*domain.TeamUser, error)
	ListByTeam(ctx context.Context, teamID uint) ([]*domain.TeamUser, error)
	ListByOrgUser(ctx context.Context, orgUserID uint) ([]*domain.TeamUser, error)
	Update(ctx context.Context, teamUser *domain.TeamUser) error
	Delete(ctx context.Context, id uint) error
	DeleteByTeamAndOrgUser(ctx context.Context, teamID, orgUserID uint) error
}

// CollectionRepository defines collection data access methods
type CollectionRepository interface {
	Create(ctx context.Context, collection *domain.Collection) error
	GetByID(ctx context.Context, id uint) (*domain.Collection, error)
	GetByUUID(ctx context.Context, uuid string) (*domain.Collection, error)
	GetByName(ctx context.Context, orgID uint, name string) (*domain.Collection, error)
	GetDefaultByOrganization(ctx context.Context, orgID uint) (*domain.Collection, error)
	ListByOrganization(ctx context.Context, orgID uint) ([]*domain.Collection, error)
	ListForUser(ctx context.Context, orgID, userID uint) ([]*domain.Collection, error)
	Update(ctx context.Context, collection *domain.Collection) error
	Delete(ctx context.Context, id uint) error
	SoftDelete(ctx context.Context, id uint) error

	// Stats
	GetItemCount(ctx context.Context, collectionID uint) (int, error)
	GetUserCount(ctx context.Context, collectionID uint) (int, error)
	GetTeamCount(ctx context.Context, collectionID uint) (int, error)
}

// CollectionUserRepository defines collection user access
type CollectionUserRepository interface {
	Create(ctx context.Context, cu *domain.CollectionUser) error
	GetByID(ctx context.Context, id uint) (*domain.CollectionUser, error)
	GetByCollectionAndOrgUser(ctx context.Context, collectionID, orgUserID uint) (*domain.CollectionUser, error)
	ListByCollection(ctx context.Context, collectionID uint) ([]*domain.CollectionUser, error)
	ListByOrgUser(ctx context.Context, orgUserID uint) ([]*domain.CollectionUser, error)
	Update(ctx context.Context, cu *domain.CollectionUser) error
	Delete(ctx context.Context, id uint) error
	DeleteByCollectionAndOrgUser(ctx context.Context, collectionID, orgUserID uint) error
}

// CollectionTeamRepository defines collection team access
type CollectionTeamRepository interface {
	Create(ctx context.Context, ct *domain.CollectionTeam) error
	GetByID(ctx context.Context, id uint) (*domain.CollectionTeam, error)
	GetByCollectionAndTeam(ctx context.Context, collectionID, teamID uint) (*domain.CollectionTeam, error)
	ListByCollection(ctx context.Context, collectionID uint) ([]*domain.CollectionTeam, error)
	ListByTeam(ctx context.Context, teamID uint) ([]*domain.CollectionTeam, error)
	Update(ctx context.Context, ct *domain.CollectionTeam) error
	Delete(ctx context.Context, id uint) error
	DeleteByCollectionAndTeam(ctx context.Context, collectionID, teamID uint) error
}

// OrganizationItemFilter represents filter options for organization items
type OrganizationItemFilter struct {
	OrganizationID uint
	CollectionID   *uint
	ItemType       *domain.ItemType
	IsFavorite     *bool
	FolderID       *uint
	AutoFill       *bool
	AutoLogin      *bool
	Search         string
	Tags           []string
	Page           int
	PerPage        int
}

// OrganizationItemRepository defines organization item data access methods
type OrganizationItemRepository interface {
	Create(ctx context.Context, item *domain.OrganizationItem) error
	GetByID(ctx context.Context, id uint) (*domain.OrganizationItem, error)
	GetByUUID(ctx context.Context, uuid string) (*domain.OrganizationItem, error)
	GetBySupportID(ctx context.Context, supportID int64) (*domain.OrganizationItem, error)
	ListByOrganization(ctx context.Context, filter OrganizationItemFilter) ([]*domain.OrganizationItem, int64, error)
	ListByCollection(ctx context.Context, collectionID uint) ([]*domain.OrganizationItem, error)
	MoveItemsToCollection(ctx context.Context, fromCollectionID uint, toCollectionID uint) error
	Update(ctx context.Context, item *domain.OrganizationItem) error
	Delete(ctx context.Context, id uint) error
	SoftDelete(ctx context.Context, id uint) error
	HardDelete(ctx context.Context, id uint) error
}

// ItemShareRepository defines item share data access methods
type ItemShareRepository interface {
	Create(ctx context.Context, share *domain.ItemShare) error
	GetByID(ctx context.Context, id uint) (*domain.ItemShare, error)
	GetByUUID(ctx context.Context, uuid string) (*domain.ItemShare, error)
	ListByItemUUID(ctx context.Context, itemUUID uuid.UUID) ([]*domain.ItemShare, error)
	ListByOwner(ctx context.Context, ownerID uint) ([]*domain.ItemShare, error)
	ListSharedWithUser(ctx context.Context, userID uint) ([]*domain.ItemShare, error)
	ListSharedWithTeam(ctx context.Context, teamID uint) ([]*domain.ItemShare, error)
	Update(ctx context.Context, share *domain.ItemShare) error
	Delete(ctx context.Context, id uint) error
	DeleteBySharedWithUser(ctx context.Context, userID uint) error
	DeleteExpired(ctx context.Context) (int64, error)
}

// UserSubscriptionRepository defines user subscription data access methods
type UserSubscriptionRepository interface {
	Create(ctx context.Context, sub *domain.UserSubscription) error
	GetByID(ctx context.Context, id uint) (*domain.UserSubscription, error)
	GetByUUID(ctx context.Context, uuid string) (*domain.UserSubscription, error)
	GetByUserID(ctx context.Context, userID uint) (*domain.UserSubscription, error)
	GetByStripeSubscriptionID(ctx context.Context, stripeSubID string) (*domain.UserSubscription, error)
	Update(ctx context.Context, sub *domain.UserSubscription) error
	Delete(ctx context.Context, id uint) error
	ExpireActiveByUserID(ctx context.Context, userID uint, endedAt time.Time) error
	ListPastDueExpired(ctx context.Context) ([]*domain.UserSubscription, error)
	ListCanceledExpired(ctx context.Context) ([]*domain.UserSubscription, error)
	ListManualExpired(ctx context.Context) ([]*domain.UserSubscription, error)
}
