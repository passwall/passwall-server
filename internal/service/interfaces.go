package service

import (
	"context"

	"github.com/passwall/passwall-server/internal/domain"
)

// Logger defines the logging interface
type Logger interface {
	Debug(msg string, keysAndValues ...interface{})
	Info(msg string, keysAndValues ...interface{})
	Infof(format string, args ...interface{})
	Warn(msg string, keysAndValues ...interface{})
	Error(msg string, keysAndValues ...interface{})
}

// AuthService defines the business logic for authentication
type AuthService interface {
	SignUp(ctx context.Context, req *domain.SignUpRequest) (*domain.User, error)
	SignIn(ctx context.Context, creds *domain.Credentials) (*domain.AuthResponse, error)
	PreLogin(ctx context.Context, email string) (*domain.PreLoginResponse, error)
	ChangeMasterPassword(ctx context.Context, req *domain.ChangeMasterPasswordRequest) error
	RefreshToken(ctx context.Context, refreshToken string) (*domain.TokenDetails, error)
	ValidateToken(ctx context.Context, token string) (*domain.TokenClaims, error)
	// SignOut revokes only the current session (device), not all sessions.
	// Use token UUID (from JWT claims) to locate and revoke the session.
	SignOut(ctx context.Context, tokenUUID string) error
	ValidateSchema(ctx context.Context, schema string) error
}

// NOTE: Legacy service interfaces removed (Login, BankAccount, CreditCard, Note, Email, Server)
// All item types now use ItemService with flexible items architecture

// UserService defines the business logic for users
type UserService interface {
	GetByID(ctx context.Context, id uint) (*domain.User, error)
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
	List(ctx context.Context) ([]*domain.User, error)
	Create(ctx context.Context, user *domain.User) error
	CreateByAdmin(ctx context.Context, req *domain.CreateUserByAdminRequest) (*domain.User, error)
	Update(ctx context.Context, id uint, user *domain.User) error
	Delete(ctx context.Context, id uint, schema string) error
	ChangeMasterPassword(ctx context.Context, req *domain.ChangeMasterPasswordRequest) error
	
	// Ownership management
	CheckOwnership(ctx context.Context, userID uint) (*domain.OwnershipCheckResult, error)
	TransferOwnership(ctx context.Context, req *domain.TransferOwnershipRequest) error
	DeleteWithOrganizations(ctx context.Context, userID uint, organizationIDs []uint, schema string) error
}

// UserNotificationPreferencesService defines business logic for notification preferences.
type UserNotificationPreferencesService interface {
	GetForUser(ctx context.Context, userID uint) (*domain.UserNotificationPreferences, error)
	UpdateForUser(ctx context.Context, userID uint, req *domain.UpdateUserNotificationPreferencesRequest) (*domain.UserNotificationPreferences, error)
}

// UserAppearancePreferencesService defines business logic for appearance preferences.
type UserAppearancePreferencesService interface {
	GetForUser(ctx context.Context, userID uint) (*domain.UserAppearancePreferences, error)
	UpdateForUser(ctx context.Context, userID uint, req *domain.UpdateUserAppearancePreferencesRequest) (*domain.UserAppearancePreferences, error)
}

// VerificationService defines the business logic for email verification
type VerificationService interface {
	GenerateCode(ctx context.Context, email string) (string, error)
	VerifyCode(ctx context.Context, email, code string) error
	ResendCode(ctx context.Context, email string) (string, error)
	CleanupExpiredCodes(ctx context.Context) error
}

// ExcludedDomainService defines the business logic for excluded domains
type ExcludedDomainService interface {
	Create(ctx context.Context, userID uint, req *domain.CreateExcludedDomainRequest) (*domain.ExcludedDomain, error)
	GetByUserID(ctx context.Context, userID uint) ([]*domain.ExcludedDomain, error)
	Delete(ctx context.Context, id uint, userID uint) error
	DeleteByDomain(ctx context.Context, userID uint, domain string) error
	IsExcluded(ctx context.Context, userID uint, domain string) (bool, error)
}

// FolderService defines the business logic for folders
type FolderService interface {
	Create(ctx context.Context, userID uint, req *domain.CreateFolderRequest) (*domain.Folder, error)
	GetByUserID(ctx context.Context, userID uint) ([]*domain.Folder, error)
	Update(ctx context.Context, id uint, userID uint, req *domain.UpdateFolderRequest) (*domain.Folder, error)
	Delete(ctx context.Context, schema string, id uint, userID uint) error
}

// OrganizationService defines the business logic for organizations
type OrganizationService interface {
	Create(ctx context.Context, userID uint, req *domain.CreateOrganizationRequest) (*domain.Organization, error)
	GetByID(ctx context.Context, id uint, userID uint) (*domain.Organization, error)
	List(ctx context.Context, userID uint) ([]*domain.Organization, error)
	Update(ctx context.Context, id uint, userID uint, req *domain.UpdateOrganizationRequest) (*domain.Organization, error)
	Delete(ctx context.Context, id uint, userID uint) error
	
	// Member management
	InviteUser(ctx context.Context, orgID uint, inviterUserID uint, req *domain.InviteUserToOrgRequest) (*domain.OrganizationUser, error)
	GetMembers(ctx context.Context, orgID uint, requestingUserID uint) ([]*domain.OrganizationUser, error)
	GetMembership(ctx context.Context, userID uint, orgID uint) (*domain.OrganizationUser, error)
	UpdateMemberRole(ctx context.Context, orgID, orgUserID uint, requestingUserID uint, req *domain.UpdateOrgUserRoleRequest) error
	RemoveMember(ctx context.Context, orgID, orgUserID uint, requestingUserID uint) error
	AcceptInvitation(ctx context.Context, orgUserID uint, userID uint) error
	AddExistingMember(ctx context.Context, orgUser *domain.OrganizationUser) error
	DeclineInvitationForUser(ctx context.Context, orgID uint, userID uint) error
	
	// Statistics
	GetMemberCount(ctx context.Context, orgID uint) (int, error)
	GetCollectionCount(ctx context.Context, orgID uint) (int, error)
}

// TeamService defines the business logic for teams
type TeamService interface {
	Create(ctx context.Context, orgID uint, userID uint, req *domain.CreateTeamRequest) (*domain.Team, error)
	GetByID(ctx context.Context, id uint, userID uint) (*domain.Team, error)
	ListByOrganization(ctx context.Context, orgID uint, userID uint) ([]*domain.Team, error)
	Update(ctx context.Context, id uint, userID uint, req *domain.UpdateTeamRequest) (*domain.Team, error)
	Delete(ctx context.Context, id uint, userID uint) error
	
	// Member management
	AddMember(ctx context.Context, teamID uint, userID uint, req *domain.AddTeamUserRequest) error
	GetMembers(ctx context.Context, teamID uint, userID uint) ([]*domain.TeamUser, error)
	UpdateMember(ctx context.Context, teamID uint, teamUserID uint, userID uint, req *domain.UpdateTeamUserRequest) error
	RemoveMember(ctx context.Context, teamID uint, teamUserID uint, userID uint) error
}

// CollectionService defines the business logic for collections
type CollectionService interface {
	Create(ctx context.Context, orgID uint, userID uint, req *domain.CreateCollectionRequest) (*domain.Collection, error)
	GetByID(ctx context.Context, id uint, userID uint) (*domain.Collection, error)
	ListByOrganization(ctx context.Context, orgID uint, userID uint) ([]*domain.Collection, error)
	ListForUser(ctx context.Context, orgID uint, userID uint) ([]*domain.Collection, error)
	Update(ctx context.Context, id uint, userID uint, req *domain.UpdateCollectionRequest) (*domain.Collection, error)
	Delete(ctx context.Context, id uint, userID uint) error
	
	// Access management
	GrantUserAccess(ctx context.Context, collectionID uint, orgUserID uint, requestingUserID uint, req *domain.GrantCollectionAccessRequest) error
	GrantTeamAccess(ctx context.Context, collectionID uint, teamID uint, requestingUserID uint, req *domain.GrantCollectionAccessRequest) error
	RevokeUserAccess(ctx context.Context, collectionID uint, orgUserID uint, requestingUserID uint) error
	RevokeTeamAccess(ctx context.Context, collectionID uint, teamID uint, requestingUserID uint) error
	GetUserAccess(ctx context.Context, collectionID uint, requestingUserID uint) ([]*domain.CollectionUser, error)
	GetTeamAccess(ctx context.Context, collectionID uint, requestingUserID uint) ([]*domain.CollectionTeam, error)
}

// OrganizationItemService defines the business logic for organization items (shared vault)
type OrganizationItemService interface {
	Create(ctx context.Context, orgID, userID uint, req *CreateOrgItemRequest) (*domain.OrganizationItem, error)
	GetByID(ctx context.Context, id, userID uint) (*domain.OrganizationItem, error)
	ListByCollection(ctx context.Context, collectionID, userID uint) ([]*domain.OrganizationItem, error)
	Update(ctx context.Context, id, userID uint, req *UpdateOrgItemRequest) (*domain.OrganizationItem, error)
	Delete(ctx context.Context, id, userID uint) (*domain.OrganizationItem, error)
}

// PaymentService defines the business logic for Stripe payments
type PaymentService interface {
	// Checkout & Subscriptions
	CreateCheckoutSession(ctx context.Context, orgID, userID uint, plan, billingCycle, ipAddress, userAgent string) (string, error)
	HandleWebhook(ctx context.Context, payload []byte, signature string) error
	
	// Subscription Management
	GetBillingInfo(ctx context.Context, orgID uint) (*domain.BillingInfo, error)
	SyncSubscription(ctx context.Context, orgID uint) error // Manually sync subscription from Stripe
}
