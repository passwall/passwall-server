package domain

import (
	"time"

	uuid "github.com/satori/go.uuid"
)

// OrganizationPlan represents the subscription plan "family" at the product level.
//
// NOTE: This is a presentation/UX concept. Source-of-truth entitlements MUST come
// from subscriptions + plans.
type OrganizationPlan string

const (
	PlanFree       OrganizationPlan = "free"
	PlanPremium    OrganizationPlan = "premium" // Premium plan does NOT use organizations
	PlanFamily     OrganizationPlan = "family"
	PlanTeam       OrganizationPlan = "team"
	PlanBusiness   OrganizationPlan = "business"
	PlanEnterprise OrganizationPlan = "enterprise"
)

// Organization represents a team/company organization
type Organization struct {
	ID        uint      `gorm:"primary_key" json:"id"`
	UUID      uuid.UUID `gorm:"type:uuid;not null" json:"uuid"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Organization info
	Name         string `json:"name" gorm:"type:varchar(255);not null"`
	BillingEmail string `json:"billing_email" gorm:"type:varchar(255);not null"`

	// System defaults
	// IsDefault marks the user's personal/default organization created at signup.
	// Default organizations cannot be deleted by their owner (but can be deleted by admin flows).
	IsDefault bool `json:"is_default" gorm:"not null;default:false"`

	// Creator snapshot (denormalized).
	// Users can be hard-deleted, so we keep immutable creator identity fields here.
	CreatedByUserID    *uint   `json:"created_by_user_id,omitempty" gorm:"index"`
	CreatedByUserEmail *string `json:"created_by_user_email,omitempty" gorm:"type:varchar(255)"`
	CreatedByUserName  *string `json:"created_by_user_name,omitempty" gorm:"type:varchar(255)"`

	// Note: Plan limits are NOT stored here - they come from subscriptions + plans tables
	// Use service methods to get plan limits via JOIN

	// Encryption
	// Organization symmetric key (AES-256) encrypted with owner's User Key
	EncryptedOrgKey string `json:"-" gorm:"type:text;not null"`

	// RSA key pair for organization (optional, for advanced key management)
	OrgPublicKey       *string `json:"-" gorm:"type:text"` // RSA-2048 public key (PEM)
	OrgPrivateKeyEnc   *string `json:"-" gorm:"type:text"` // RSA private key encrypted with recovery key
	KeyRotationCounter int     `json:"key_rotation_counter" gorm:"default:0"`

	// Status
	Status              OrganizationStatus `json:"status" gorm:"type:varchar(20);not null;default:'active'"`
	IsActive            bool               `json:"is_active" gorm:"default:true"`
	SuspendedAt         *time.Time         `json:"suspended_at,omitempty"`
	DeletedAt           *time.Time         `json:"deleted_at,omitempty" gorm:"index"`
	ScheduledDeletionAt *time.Time         `json:"scheduled_deletion_at,omitempty"`

	// Billing & Stripe Integration
	StripeCustomerID *string `json:"stripe_customer_id,omitempty" gorm:"type:varchar(255);index"`

	// Stats (runtime calculated, not stored in DB)
	MemberCount     *int `json:"member_count,omitempty" gorm:"-"`
	TeamCount       *int `json:"team_count,omitempty" gorm:"-"`
	CollectionCount *int `json:"collection_count,omitempty" gorm:"-"`
	ItemCount       *int `json:"item_count,omitempty" gorm:"-"`

	// Associations (not loaded by default)
	Members     []OrganizationUser `json:"members,omitempty" gorm:"foreignKey:OrganizationID"`
	Teams       []Team             `json:"teams,omitempty" gorm:"foreignKey:OrganizationID"`
	Collections []Collection       `json:"collections,omitempty" gorm:"foreignKey:OrganizationID"`
}

// TableName specifies the table name
func (Organization) TableName() string {
	return "organizations"
}

// OrganizationStatus represents the status of an organization
type OrganizationStatus string

const (
	OrgStatusActive               OrganizationStatus = "active"
	OrgStatusSuspended            OrganizationStatus = "suspended"
	OrgStatusScheduledForDeletion OrganizationStatus = "scheduled_for_deletion"
	OrgStatusDeleted              OrganizationStatus = "deleted"
)

// OrganizationRole represents a user's role in an organization
type OrganizationRole string

const (
	OrgRoleOwner   OrganizationRole = "owner"   // Full control, billing, delete org
	OrgRoleAdmin   OrganizationRole = "admin"   // Manage users, collections, all items
	OrgRoleManager OrganizationRole = "manager" // Manage specific teams/collections
	OrgRoleMember  OrganizationRole = "member"  // Access assigned collections only
	OrgRoleBilling OrganizationRole = "billing" // Billing management only
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

	OrganizationID uint `json:"organization_id" gorm:"not null;index;constraint:OnDelete:CASCADE"`
	UserID         uint `json:"user_id" gorm:"not null;index;constraint:OnDelete:CASCADE"`

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
	ID                 uint               `json:"id"`
	UUID               uuid.UUID          `json:"uuid"`
	Name               string             `json:"name"`
	BillingEmail       string             `json:"billing_email"`
	IsDefault          bool               `json:"is_default"`
	CreatedByUserID    *uint              `json:"created_by_user_id,omitempty"`
	CreatedByUserEmail *string            `json:"created_by_user_email,omitempty"`
	CreatedByUserName  *string            `json:"created_by_user_name,omitempty"`
	Plan               OrganizationPlan   `json:"plan"`
	MaxUsers           int                `json:"max_users"`
	MaxCollections     int                `json:"max_collections"`
	Status             OrganizationStatus `json:"status"`
	IsActive           bool               `json:"is_active"`
	CreatedAt          time.Time          `json:"created_at"`
	UpdatedAt          time.Time          `json:"updated_at"`

	// Subscription (optional)
	Subscription *SubscriptionDTO `json:"subscription,omitempty"`

	// Stats (optional)
	MemberCount     *int `json:"member_count,omitempty"`
	TeamCount       *int `json:"team_count,omitempty"`
	CollectionCount *int `json:"collection_count,omitempty"`
	ItemCount       *int `json:"item_count,omitempty"`

	// Encrypted org key (safe to send - user's own copy, encrypted with their User Key)
	EncryptedOrgKey string `json:"encrypted_org_key,omitempty"`
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

// ToOrganizationDTOWithSubscription converts Organization to DTO and derives plan/limits
// from the provided subscription (source of truth).
func ToOrganizationDTOWithSubscription(org *Organization, sub *Subscription) *OrganizationDTO {
	dto := ToOrganizationDTO(org)
	if dto == nil {
		return nil
	}

	if sub == nil {
		return dto
	}

	dto.Subscription = ToSubscriptionDTO(sub)

	if sub.Plan == nil {
		return dto
	}

	planCode := sub.Plan.Code

	// Map plan code like "business-yearly" -> "business"
	if idx := len(planCode) - len("-monthly"); idx > 0 && planCode[idx:] == "-monthly" {
		dto.Plan = OrganizationPlan(planCode[:idx])
	} else if idx := len(planCode) - len("-yearly"); idx > 0 && planCode[idx:] == "-yearly" {
		dto.Plan = OrganizationPlan(planCode[:idx])
	} else if planCode != "" {
		dto.Plan = OrganizationPlan(planCode)
	}

	// Limits: nil means unlimited. UI treats 999 as unlimited.
	if sub.Plan.MaxUsers != nil {
		dto.MaxUsers = *sub.Plan.MaxUsers
	} else {
		dto.MaxUsers = 999
	}

	if sub.Plan.MaxCollections != nil {
		dto.MaxCollections = *sub.Plan.MaxCollections
	} else {
		dto.MaxCollections = 999
	}

	return dto
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

	dto := &OrganizationDTO{
		ID:                 org.ID,
		UUID:               org.UUID,
		Name:               org.Name,
		BillingEmail:       org.BillingEmail,
		IsDefault:          org.IsDefault,
		CreatedByUserID:    org.CreatedByUserID,
		CreatedByUserEmail: org.CreatedByUserEmail,
		CreatedByUserName:  org.CreatedByUserName,
		// Default to free plan - service layer should populate these from subscription
		Plan:            PlanFree,
		MaxUsers:        1,
		MaxCollections:  10,
		Status:          org.Status,
		IsActive:        org.IsActive,
		CreatedAt:       org.CreatedAt,
		UpdatedAt:       org.UpdatedAt,
		MemberCount:     org.MemberCount,
		TeamCount:       org.TeamCount,
		CollectionCount: org.CollectionCount,
		ItemCount:       org.ItemCount,
		EncryptedOrgKey: org.EncryptedOrgKey, // User's copy (safe to send)
	}

	return dto
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

// BillingInfo contains billing and subscription information
type BillingInfo struct {
	Organization *OrganizationDTO `json:"organization"`
	Subscription *SubscriptionDTO `json:"subscription,omitempty"`

	// Current usage
	CurrentUsers       int `json:"current_users"`
	CurrentCollections int `json:"current_collections"`
	CurrentItems       int `json:"current_items"`

	// Invoices
	Invoices []*InvoiceDTO `json:"invoices,omitempty"`
}
