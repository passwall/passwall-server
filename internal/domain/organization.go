package domain

import (
	"time"

	uuid "github.com/satori/go.uuid"
)

// OrganizationPlan represents the subscription plan
type OrganizationPlan string

const (
	PlanFree       OrganizationPlan = "free"       // 2 users, 2 collections
	PlanBusiness   OrganizationPlan = "business"   // Unlimited users, unlimited collections
	PlanEnterprise OrganizationPlan = "enterprise" // Business + SSO + LDAP
)

// Organization represents a team/company organization
type Organization struct {
	ID        uint      `gorm:"primary_key" json:"id"`
	UUID      uuid.UUID `gorm:"type:uuid;not null" json:"uuid"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Organization info
	Name         string           `json:"name" gorm:"type:varchar(255);not null"`
	BillingEmail string           `json:"billing_email" gorm:"type:varchar(255);not null"`
	Plan         OrganizationPlan `json:"plan" gorm:"type:varchar(50);not null;default:'free'"`

	// Plan limits
	MaxUsers       int `json:"max_users" gorm:"default:5"`
	MaxCollections int `json:"max_collections" gorm:"default:10"`

	// Encryption
	// Organization symmetric key (AES-256) encrypted with owner's User Key
	EncryptedOrgKey string `json:"-" gorm:"type:text;not null"`

	// RSA key pair for organization (optional, for advanced key management)
	OrgPublicKey       *string `json:"-" gorm:"type:text"` // RSA-2048 public key (PEM)
	OrgPrivateKeyEnc   *string `json:"-" gorm:"type:text"` // RSA private key encrypted with recovery key
	KeyRotationCounter int     `json:"key_rotation_counter" gorm:"default:0"`

	// Status
	IsActive    bool       `json:"is_active" gorm:"default:true"`
	SuspendedAt *time.Time `json:"suspended_at,omitempty"`

	// Billing
	SubscriptionID     *string    `json:"subscription_id,omitempty" gorm:"type:varchar(255)"`
	SubscriptionStatus *string    `json:"subscription_status,omitempty" gorm:"type:varchar(50)"`
	TrialEndDate       *time.Time `json:"trial_end_date,omitempty"`

	// Associations (not loaded by default)
	Members     []OrganizationUser `json:"members,omitempty" gorm:"foreignKey:OrganizationID"`
	Teams       []Team             `json:"teams,omitempty" gorm:"foreignKey:OrganizationID"`
	Collections []Collection       `json:"collections,omitempty" gorm:"foreignKey:OrganizationID"`
}

// TableName specifies the table name
func (Organization) TableName() string {
	return "organizations"
}

// OrganizationRole represents a user's role in an organization
type OrganizationRole string

const (
	OrgRoleOwner   OrganizationRole = "owner"   // Full control, billing, delete org
	OrgRoleAdmin   OrganizationRole = "admin"   // Manage users, collections, all items
	OrgRoleManager OrganizationRole = "manager" // Manage specific teams/collections
	OrgRoleMember  OrganizationRole = "member"  // Access assigned collections only
)

// OrganizationUserStatus represents the status of a user's membership
type OrganizationUserStatus string

const (
	OrgUserStatusInvited   OrganizationUserStatus = "invited"
	OrgUserStatusAccepted  OrganizationUserStatus = "accepted"
	OrgUserStatusConfirmed OrganizationUserStatus = "confirmed"
	OrgUserStatusSuspended OrganizationUserStatus = "suspended"
)

// OrganizationUser represents a user's membership in an organization
type OrganizationUser struct {
	ID        uint      `gorm:"primary_key" json:"id"`
	UUID      uuid.UUID `gorm:"type:uuid;not null" json:"uuid"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	OrganizationID uint `json:"organization_id" gorm:"not null;index"`
	UserID         uint `json:"user_id" gorm:"not null;index"`

	// Role in organization
	Role OrganizationRole `json:"role" gorm:"type:varchar(20);not null;default:'member'"`

	// Organization key wrapped for this user
	// Encrypted with user's RSA public key (or User Key if RSA not available)
	EncryptedOrgKey string `json:"-" gorm:"type:text;not null"`

	// Permissions
	AccessAll   bool   `json:"access_all" gorm:"default:false"` // Access all collections
	Permissions string `json:"permissions,omitempty" gorm:"type:text"`

	// Status
	Status     OrganizationUserStatus `json:"status" gorm:"type:varchar(20);default:'invited'"`
	InvitedAt  *time.Time             `json:"invited_at,omitempty"`
	AcceptedAt *time.Time             `json:"accepted_at,omitempty"`

	// External ID for LDAP/AD sync
	ExternalID *string `json:"external_id,omitempty" gorm:"type:varchar(255);index"`

	// Associations
	Organization *Organization `json:"organization,omitempty" gorm:"foreignKey:OrganizationID"`
	User         *User         `json:"user,omitempty" gorm:"foreignKey:UserID"`
}

// TableName specifies the table name
func (OrganizationUser) TableName() string {
	return "organization_users"
}

// IsOwner checks if the user is an owner
func (ou *OrganizationUser) IsOwner() bool {
	return ou.Role == OrgRoleOwner
}

// IsAdmin checks if the user is an admin or owner
func (ou *OrganizationUser) IsAdmin() bool {
	return ou.Role == OrgRoleOwner || ou.Role == OrgRoleAdmin
}

// CanManageUsers checks if the user can manage other users
func (ou *OrganizationUser) CanManageUsers() bool {
	return ou.Role == OrgRoleOwner || ou.Role == OrgRoleAdmin
}

// CanManageCollections checks if the user can manage collections
func (ou *OrganizationUser) CanManageCollections() bool {
	return ou.Role == OrgRoleOwner || ou.Role == OrgRoleAdmin || ou.Role == OrgRoleManager
}

// OrganizationDTO for API responses
type OrganizationDTO struct {
	ID             uint             `json:"id"`
	UUID           uuid.UUID        `json:"uuid"`
	Name           string           `json:"name"`
	BillingEmail   string           `json:"billing_email"`
	Plan           OrganizationPlan `json:"plan"`
	MaxUsers       int              `json:"max_users"`
	MaxCollections int              `json:"max_collections"`
	IsActive       bool             `json:"is_active"`
	CreatedAt      time.Time        `json:"created_at"`
	UpdatedAt      time.Time        `json:"updated_at"`

	// Stats (optional)
	MemberCount     *int `json:"member_count,omitempty"`
	TeamCount       *int `json:"team_count,omitempty"`
	CollectionCount *int `json:"collection_count,omitempty"`
}

// OrganizationUserDTO for API responses
type OrganizationUserDTO struct {
	ID             uint                   `json:"id"`
	UUID           uuid.UUID              `json:"uuid"`
	OrganizationID uint                   `json:"organization_id"`
	UserID         uint                   `json:"user_id"`
	UserEmail      string                 `json:"user_email"`
	UserName       string                 `json:"user_name"`
	Role           OrganizationRole       `json:"role"`
	AccessAll      bool                   `json:"access_all"`
	Status         OrganizationUserStatus `json:"status"`
	InvitedAt      *time.Time             `json:"invited_at,omitempty"`
	AcceptedAt     *time.Time             `json:"accepted_at,omitempty"`
	CreatedAt      time.Time              `json:"created_at"`
}

// CreateOrganizationRequest for API requests
type CreateOrganizationRequest struct {
	Name            string `json:"name" validate:"required,max=255"`
	BillingEmail    string `json:"billing_email" validate:"required,email"`
	Plan            string `json:"plan" validate:"omitempty,oneof=free business enterprise"`
	EncryptedOrgKey string `json:"encrypted_org_key" validate:"required"` // Owner's copy of org key
}

// UpdateOrganizationRequest for API requests
type UpdateOrganizationRequest struct {
	Name         *string `json:"name,omitempty" validate:"omitempty,max=255"`
	BillingEmail *string `json:"billing_email,omitempty" validate:"omitempty,email"`
}

// InviteUserToOrgRequest for inviting users
type InviteUserToOrgRequest struct {
	Email           string           `json:"email" validate:"required,email"`
	Role            OrganizationRole `json:"role" validate:"required,oneof=owner admin manager member"`
	EncryptedOrgKey string           `json:"encrypted_org_key" validate:"required"` // Org key wrapped for invitee
	AccessAll       bool             `json:"access_all"`
	Collections     []uint           `json:"collections,omitempty"` // Collection IDs to grant access
}

// UpdateOrgUserRoleRequest for updating user role
type UpdateOrgUserRoleRequest struct {
	Role      OrganizationRole `json:"role" validate:"required,oneof=owner admin manager member"`
	AccessAll *bool            `json:"access_all,omitempty"`
}

// ToOrganizationDTO converts Organization to DTO
func ToOrganizationDTO(org *Organization) *OrganizationDTO {
	if org == nil {
		return nil
	}

	return &OrganizationDTO{
		ID:             org.ID,
		UUID:           org.UUID,
		Name:           org.Name,
		BillingEmail:   org.BillingEmail,
		Plan:           org.Plan,
		MaxUsers:       org.MaxUsers,
		MaxCollections: org.MaxCollections,
		IsActive:       org.IsActive,
		CreatedAt:      org.CreatedAt,
		UpdatedAt:      org.UpdatedAt,
	}
}

// ToOrganizationUserDTO converts OrganizationUser to DTO
func ToOrganizationUserDTO(ou *OrganizationUser) *OrganizationUserDTO {
	if ou == nil {
		return nil
	}

	dto := &OrganizationUserDTO{
		ID:             ou.ID,
		UUID:           ou.UUID,
		OrganizationID: ou.OrganizationID,
		UserID:         ou.UserID,
		Role:           ou.Role,
		AccessAll:      ou.AccessAll,
		Status:         ou.Status,
		InvitedAt:      ou.InvitedAt,
		AcceptedAt:     ou.AcceptedAt,
		CreatedAt:      ou.CreatedAt,
	}

	// Add user info if loaded
	if ou.User != nil {
		dto.UserEmail = ou.User.Email
		dto.UserName = ou.User.Name
	}

	return dto
}

