package service

import (
	"context"

	"github.com/passwall/passwall-server/internal/domain"
)

// Logger defines the logging interface
type Logger interface {
	Debug(msg string, keysAndValues ...interface{})
	Info(msg string, keysAndValues ...interface{})
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
	SignOut(ctx context.Context, userID int) error
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
	Delete(ctx context.Context, id uint, userID uint) error
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
	UpdateMemberRole(ctx context.Context, orgID, orgUserID uint, requestingUserID uint, req *domain.UpdateOrgUserRoleRequest) error
	RemoveMember(ctx context.Context, orgID, orgUserID uint, requestingUserID uint) error
	AcceptInvitation(ctx context.Context, orgUserID uint, userID uint) error
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
