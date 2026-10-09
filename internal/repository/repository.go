package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/passwall/passwall-server/internal/domain"
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
	Search      string
	OwnerUserID uint
	Limit       int
	Offset      int
	Sort        string
	Order       string
	// Organization-only admin filters.
	SearchOwners          bool       // also match owner email/name
	ManualGrantEndsBefore *time.Time // only open manual grants ending before this time
	SortByAccessEndAsc    bool       // soonest access end first, organizations without one last
}

// OrganizationCounts holds usage counters for one organization.
type OrganizationCounts struct {
	Members     int
	Teams       int
	Collections int
	Items       int
}

// ListResult represents list query results with pagination info
type ListResult struct {
	Total    int64
	Filtered int64
}

// UserRepository defines user data access methods
type UserRepository interface {
	GetByID(ctx context.Context, id uint) (*domain.User, error)
	GetByUUID(ctx context.Context, uuid string) (*domain.User, error)
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
	List(ctx context.Context, filter ListFilter) ([]*domain.User, *ListResult, error)
	CountByRoleID(ctx context.Context, roleID uint) (int64, error)
	Create(ctx context.Context, user *domain.User) error
	Update(ctx context.Context, user *domain.User) error
	Delete(ctx context.Context, id uint) error
	Migrate() error
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
	CountActiveSessionsByUserID(ctx context.Context, userID int) (int, error)
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

// AccountDeletionTokenRepository defines account deletion token data access methods.
type AccountDeletionTokenRepository interface {
	Create(ctx context.Context, token *domain.AccountDeletionToken) error
	GetByUUID(ctx context.Context, tokenUUID string) (*domain.AccountDeletionToken, error)
	DeleteByUUID(ctx context.Context, tokenUUID string) error
	DeleteByUserID(ctx context.Context, userID uint) error
	DeleteExpired(ctx context.Context) (int64, error)
	Migrate() error
}

// OrganizationRepository defines organization data access methods
type OrganizationRepository interface {
	Create(ctx context.Context, org *domain.Organization) error
	GetByID(ctx context.Context, id uint) (*domain.Organization, error)
	GetByUUID(ctx context.Context, uuid string) (*domain.Organization, error)
	GetByPublicID(ctx context.Context, publicID string) (*domain.Organization, error)
	GetDefaultByOwnerID(ctx context.Context, ownerUserID uint) (*domain.Organization, error)
	List(ctx context.Context, filter ListFilter) ([]*domain.Organization, *ListResult, error)
	ListForUser(ctx context.Context, userID uint) ([]*domain.Organization, error)
	Update(ctx context.Context, org *domain.Organization) error
	Delete(ctx context.Context, id uint) error
	PurgePersonal(ctx context.Context, id uint) error

	// Stats
	GetMemberCount(ctx context.Context, orgID uint) (int, error)
	GetTeamCount(ctx context.Context, orgID uint) (int, error)
	GetCollectionCount(ctx context.Context, orgID uint) (int, error)
	GetItemCount(ctx context.Context, orgID uint) (int, error)
	// GetCountsByIDs batches the counters above for list views.
	GetCountsByIDs(ctx context.Context, orgIDs []uint) (map[uint]OrganizationCounts, error)
}

// OrganizationUserRepository defines organization user data access methods
type OrganizationUserRepository interface {
	Create(ctx context.Context, orgUser *domain.OrganizationUser) error
	GetByID(ctx context.Context, id uint) (*domain.OrganizationUser, error)
	GetByUUID(ctx context.Context, uuid string) (*domain.OrganizationUser, error)
	GetByOrgAndUser(ctx context.Context, orgID, userID uint) (*domain.OrganizationUser, error)
	GetActiveByOrgAndUser(ctx context.Context, orgID, userID uint) (*domain.OrganizationUser, error)
	ListByOrganization(ctx context.Context, orgID uint) ([]*domain.OrganizationUser, error)
	// ListOwnersByOrganizationIDs returns the earliest owner (with User) per organization.
	ListOwnersByOrganizationIDs(ctx context.Context, orgIDs []uint) (map[uint]*domain.OrganizationUser, error)
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
	OrganizationID        uint
	CollectionID          *uint
	ItemType              *domain.ItemType
	IsFavorite            *bool
	FolderID              *uint
	AutoFill              *bool
	AutoLogin             *bool
	Search                string
	Tags                  []string
	Page                  int
	PerPage               int
	RestrictToCollections bool
	AllowedCollectionIDs  []uint
	AllowOrgWide          bool
}

type OrganizationItemV2Filter struct {
	OrganizationID        uint
	AfterRevision         int64
	AfterID               uint
	SinceRevision         int64
	UpToRevision          int64
	Limit                 int
	RestrictToCollections bool
	AllowedCollectionIDs  []uint
}

// OrganizationItemRepository defines organization item data access methods
type OrganizationItemRepository interface {
	Create(ctx context.Context, item *domain.OrganizationItem) error
	GetByID(ctx context.Context, id uint) (*domain.OrganizationItem, error)
	GetByUUID(ctx context.Context, uuid string) (*domain.OrganizationItem, error)
	GetBySupportID(ctx context.Context, supportID int64) (*domain.OrganizationItem, error)
	ListByOrganization(ctx context.Context, filter OrganizationItemFilter) ([]*domain.OrganizationItem, int64, error)
	ListV2(ctx context.Context, filter OrganizationItemV2Filter) ([]*domain.OrganizationItem, int64, error)
	ListByCollection(ctx context.Context, collectionID uint) ([]*domain.OrganizationItem, error)
	MoveItemsToCollection(ctx context.Context, fromCollectionID uint, toCollectionID uint) error
	CountByOrganizationID(ctx context.Context, orgID uint) (int, error)
	Update(ctx context.Context, item *domain.OrganizationItem) error
	Delete(ctx context.Context, id uint) error
	SoftDelete(ctx context.Context, id uint) error
	HardDelete(ctx context.Context, id uint) error
	ClearCreatorByUserID(ctx context.Context, userID uint) error
}

// SSOConnectionRepository defines SSO connection data access methods
type SSOConnectionRepository interface {
	Create(ctx context.Context, conn *domain.SSOConnection) error
	GetByID(ctx context.Context, id uint) (*domain.SSOConnection, error)
	GetByUUID(ctx context.Context, uuid string) (*domain.SSOConnection, error)
	GetAnyByDomain(ctx context.Context, domain string) (*domain.SSOConnection, error)
	GetByDomain(ctx context.Context, domain string) (*domain.SSOConnection, error)
	GetByOrganizationID(ctx context.Context, orgID uint) (*domain.SSOConnection, error)
	ListByOrganization(ctx context.Context, orgID uint) ([]*domain.SSOConnection, error)
	Update(ctx context.Context, conn *domain.SSOConnection) error
	Delete(ctx context.Context, id uint) error
}

// SSOStateRepository defines SSO transient state data access methods
type SSOStateRepository interface {
	Create(ctx context.Context, state *domain.SSOState) error
	GetByState(ctx context.Context, state string) (*domain.SSOState, error)
	Delete(ctx context.Context, id uint) error
	DeleteExpired(ctx context.Context) (int64, error)
}

// SCIMTokenRepository defines SCIM token data access methods
type SCIMTokenRepository interface {
	Create(ctx context.Context, token *domain.SCIMToken) error
	GetByID(ctx context.Context, id uint) (*domain.SCIMToken, error)
	GetByTokenHash(ctx context.Context, tokenHash string) (*domain.SCIMToken, error)
	ListByOrganization(ctx context.Context, orgID uint) ([]*domain.SCIMToken, error)
	Update(ctx context.Context, token *domain.SCIMToken) error
	Delete(ctx context.Context, id uint) error
}

// OrganizationPolicyRepository defines organization policy data access methods
type OrganizationPolicyRepository interface {
	Create(ctx context.Context, policy *domain.OrganizationPolicy) error
	GetByID(ctx context.Context, id uint) (*domain.OrganizationPolicy, error)
	GetByOrgAndType(ctx context.Context, orgID uint, policyType domain.PolicyType) (*domain.OrganizationPolicy, error)
	ListByOrganization(ctx context.Context, orgID uint) ([]*domain.OrganizationPolicy, error)
	ListEnabledByOrganization(ctx context.Context, orgID uint) ([]*domain.OrganizationPolicy, error)
	Update(ctx context.Context, policy *domain.OrganizationPolicy) error
	Delete(ctx context.Context, id uint) error
}

type OrganizationEntitlementOverrideRepository interface {
	ListActiveByOrganization(ctx context.Context, orgID uint, now time.Time) ([]*domain.OrganizationEntitlementOverride, error)
}

type WebhookEventRepository interface {
	Create(ctx context.Context, event *domain.WebhookEvent) error
	GetByStripeEventID(ctx context.Context, stripeEventID string) (*domain.WebhookEvent, error)
	MarkProcessed(ctx context.Context, stripeEventID string) error
	MarkFailed(ctx context.Context, stripeEventID string, errMsg string) error
}

// EmergencyAccessRepository defines emergency access data access methods
type EmergencyAccessRepository interface {
	Create(ctx context.Context, ea *domain.EmergencyAccess) error
	GetByUUID(ctx context.Context, uuid string) (*domain.EmergencyAccess, error)
	ListByGrantor(ctx context.Context, grantorID uint) ([]*domain.EmergencyAccess, error)
	ListByGrantee(ctx context.Context, granteeID uint) ([]*domain.EmergencyAccess, error)
	ListByGranteeEmail(ctx context.Context, email string) ([]*domain.EmergencyAccess, error)
	ListConfirmedByGrantor(ctx context.Context, grantorID uint) ([]*domain.EmergencyAccess, error)
	Update(ctx context.Context, ea *domain.EmergencyAccess) error
	Delete(ctx context.Context, id uint) error
}

// SendRepository defines send data access methods
type SendRepository interface {
	Create(ctx context.Context, send *domain.Send) error
	GetByUUID(ctx context.Context, uuid string) (*domain.Send, error)
	GetByAccessID(ctx context.Context, accessID string) (*domain.Send, error)
	ListByCreator(ctx context.Context, creatorID uint) ([]*domain.Send, error)
	Update(ctx context.Context, send *domain.Send) error
	Delete(ctx context.Context, id uint) error
	SoftDelete(ctx context.Context, id uint) error
	IncrementAccessCount(ctx context.Context, id uint) error
	DeleteExpired(ctx context.Context) (int64, error)
}

// KeyEscrowRepository defines key escrow data access methods
type KeyEscrowRepository interface {
	Create(ctx context.Context, escrow *domain.KeyEscrow) error
	GetByUserAndOrg(ctx context.Context, userID, orgID uint) (*domain.KeyEscrow, error)
	ListByOrganization(ctx context.Context, orgID uint) ([]*domain.KeyEscrow, error)
	Update(ctx context.Context, escrow *domain.KeyEscrow) error
	Delete(ctx context.Context, id uint) error
	DeleteByUserAndOrg(ctx context.Context, userID, orgID uint) error
}

// OrgEscrowKeyRepository defines org escrow key data access methods
type OrgEscrowKeyRepository interface {
	Create(ctx context.Context, key *domain.OrgEscrowKey) error
	GetByOrganizationID(ctx context.Context, orgID uint) (*domain.OrgEscrowKey, error)
	Update(ctx context.Context, key *domain.OrgEscrowKey) error
	Delete(ctx context.Context, id uint) error
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

// CompatTelemetryRepository defines compatibility telemetry persistence methods.
type CompatTelemetryRepository interface {
	CreateBatch(ctx context.Context, events []*domain.CompatTelemetryEvent) error
	List(ctx context.Context, filter CompatTelemetryListFilter) ([]*domain.CompatTelemetryEvent, int64, int64, error)
	DeleteOlderThan(ctx context.Context, before time.Time) (int64, error)
	ListSummary(ctx context.Context, filter CompatTelemetryListFilter) ([]*domain.CompatTelemetrySummaryRow, int64, error)
	// ListExistingCompatKeys returns distinct (domain_etld1, page_path, event_name, error_code, flow_type, surface, succeeded) since the given time (for server-side dedupe).
	ListExistingCompatKeys(ctx context.Context, since time.Time) ([]CompatTelemetryDedupeKey, error)
}

// CompatTelemetryDedupeKey is the business key used to skip duplicate telemetry events.
type CompatTelemetryDedupeKey struct {
	DomainETLD1 string
	PagePath    string
	EventName   string
	ErrorCode   string
	FlowType    string
	Surface     string
	Succeeded   bool
}

// CompatTelemetryListFilter represents filter options for compatibility telemetry list queries.
type CompatTelemetryListFilter struct {
	Search       string
	Domain       string
	PagePath     string
	EventName    string
	FlowType     string
	Surface      string
	ErrorCode    string
	Succeeded    *bool
	Order        string
	Limit        int
	Offset       int
	CreatedAfter *time.Time // retention: only events after this time
}
