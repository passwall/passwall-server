package domain

import (
	"time"

	"github.com/google/uuid"
)

// AdminDirectorySchemaVersion is the contract version for the read-only admin directory API.
// Additive JSON fields do not change this value. Increment it only when a client would break.
const AdminDirectorySchemaVersion = 1

// AdminDirectorySubscriptionUnknown is returned when no subscription row is stored
// for the user's personal organization.
const AdminDirectorySubscriptionUnknown = "unknown"

// AdminDirectoryUser is the stable per-account record returned to service clients.
//
// New fields may be added. Clients must ignore unknown keys. This struct must never
// grow vault payloads, passwords, notes, cards, bank accounts, secrets, tokens, or keys.
// Item fields are counts only.
type AdminDirectoryUser struct {
	ID            uint                       `json:"id"`
	UUID          string                     `json:"uuid"`
	Email         string                     `json:"email"`
	EmailVerified bool                       `json:"email_verified"`
	RegisteredAt  time.Time                  `json:"registered_at"`
	LastLoginAt   *time.Time                 `json:"last_login_at"`
	Plan          *AdminDirectoryPlan        `json:"plan"`
	Subscription  AdminDirectorySubscription `json:"subscription"`

	// Vault is the personal schema item counts. Nil when that schema cannot be counted.
	Vault *AdminDirectoryItemCounts `json:"vault"`
	// OrganizationItems counts organization_items created by this account.
	OrganizationItems AdminDirectoryItemCounts `json:"organization_items"`
	OrganizationCount int                      `json:"organization_count"`
	CollectionCount   int                      `json:"collection_count"`

	// LastActivityAt is the newest stored activity that is not a sign-in.
	LastActivityAt *time.Time `json:"last_activity_at"`
	// SignInCount and ActivityCount are rows still stored in user_activities,
	// not lifetime totals. Activity cleanup deletes rows older than 90 days.
	SignInCount   int `json:"signin_count"`
	ActivityCount int `json:"activity_count"`
	// DeviceCount and ClientCount are distinct device_id and app values on stored token rows.
	DeviceCount int `json:"device_count"`
	ClientCount int `json:"client_count"`
}

// AdminDirectoryItemCounts is a count of stored vault rows. It never includes item contents.
type AdminDirectoryItemCounts struct {
	ItemCount int                       `json:"item_count"`
	ByType    AdminDirectoryItemsByType `json:"by_type"`
}

// AdminDirectoryItemsByType breaks a count down by the stored item_type enum.
// Other is any item_type value that is not one of the known constants.
type AdminDirectoryItemsByType struct {
	Password    int `json:"password"`
	SecureNote  int `json:"secure_note"`
	Card        int `json:"card"`
	BankAccount int `json:"bank_account"`
	Email       int `json:"email"`
	Server      int `json:"server"`
	Identity    int `json:"identity"`
	SSHKey      int `json:"ssh_key"`
	Address     int `json:"address"`
	Passkey     int `json:"passkey"`
	Custom      int `json:"custom"`
	Other       int `json:"other"`
}

// Add records n rows of itemType and keeps ItemCount equal to the sum of ByType.
func (c *AdminDirectoryItemCounts) Add(itemType ItemType, n int) {
	if c == nil || n <= 0 {
		return
	}
	switch itemType {
	case ItemTypePassword:
		c.ByType.Password += n
	case ItemTypeSecureNote:
		c.ByType.SecureNote += n
	case ItemTypeCard:
		c.ByType.Card += n
	case ItemTypeBankAccount:
		c.ByType.BankAccount += n
	case ItemTypeEmail:
		c.ByType.Email += n
	case ItemTypeServer:
		c.ByType.Server += n
	case ItemTypeIdentity:
		c.ByType.Identity += n
	case ItemTypeSSHKey:
		c.ByType.SSHKey += n
	case ItemTypeAddress:
		c.ByType.Address += n
	case ItemTypePasskey:
		c.ByType.Passkey += n
	case ItemTypeCustom:
		c.ByType.Custom += n
	default:
		c.ByType.Other += n
	}
	c.ItemCount += n
}

// AdminDirectoryUsage is stored usage for one account. Vault is nil when the
// personal schema cannot be counted. Other counts are zero when no rows exist.
type AdminDirectoryUsage struct {
	Vault             *AdminDirectoryItemCounts
	OrganizationItems AdminDirectoryItemCounts
	OrganizationCount int
	CollectionCount   int
	LastActivityAt    *time.Time
	SignInCount       int
	ActivityCount     int
	DeviceCount       int
	ClientCount       int
}

// AdminDirectoryPlan is the stored plan attached to the user's personal-organization subscription.
type AdminDirectoryPlan struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

// AdminDirectorySubscription reports the stored subscription state.
// Status is AdminDirectorySubscriptionUnknown when no subscription row exists.
type AdminDirectorySubscription struct {
	Status string `json:"status"`
}

// AdminDirectoryPagination describes a stable offset page.
type AdminDirectoryPagination struct {
	Limit  int   `json:"limit"`
	Offset int   `json:"offset"`
	Total  int64 `json:"total"`
}

// AdminDirectoryListResponse is the envelope for GET /api/admin/directory/users.
type AdminDirectoryListResponse struct {
	SchemaVersion int                      `json:"schema_version"`
	Data          []AdminDirectoryUser     `json:"data"`
	Pagination    AdminDirectoryPagination `json:"pagination"`
}

// AdminDirectoryUserResponse is the envelope for GET /api/admin/directory/users/:id.
type AdminDirectoryUserResponse struct {
	SchemaVersion int                `json:"schema_version"`
	Data          AdminDirectoryUser `json:"data"`
}

// NewAdminDirectoryUser maps stored account data into the directory contract.
//
// lastLogin is the newest stored sign-in activity time. A nil or zero time becomes JSON null.
// sub is the effective subscription of the user's personal organization. A nil subscription,
// or a subscription whose plan row is missing, leaves plan null and status "unknown"
// (status is the stored state when the subscription row exists).
// usage carries counts that already exist. A nil Vault stays JSON null.
func NewAdminDirectoryUser(user *User, lastLogin *time.Time, sub *Subscription, usage AdminDirectoryUsage) AdminDirectoryUser {
	dto := AdminDirectoryUser{
		Subscription:      AdminDirectorySubscription{Status: AdminDirectorySubscriptionUnknown},
		Vault:             usage.Vault,
		OrganizationItems: usage.OrganizationItems,
		OrganizationCount: usage.OrganizationCount,
		CollectionCount:   usage.CollectionCount,
		SignInCount:       usage.SignInCount,
		ActivityCount:     usage.ActivityCount,
		DeviceCount:       usage.DeviceCount,
		ClientCount:       usage.ClientCount,
	}
	if usage.LastActivityAt != nil && !usage.LastActivityAt.IsZero() {
		activityAt := usage.LastActivityAt.UTC()
		dto.LastActivityAt = &activityAt
	}
	if user == nil {
		return dto
	}

	dto.ID = user.ID
	if user.UUID != uuid.Nil {
		dto.UUID = user.UUID.String()
	} else {
		dto.UUID = uuid.Nil.String()
	}
	dto.Email = user.Email
	dto.EmailVerified = user.IsVerified
	if !user.CreatedAt.IsZero() {
		dto.RegisteredAt = user.CreatedAt.UTC()
	}
	if lastLogin != nil && !lastLogin.IsZero() {
		logged := lastLogin.UTC()
		dto.LastLoginAt = &logged
	}
	if sub == nil || sub.State == "" {
		return dto
	}
	dto.Subscription.Status = string(sub.State)
	if sub.Plan != nil && (sub.Plan.Code != "" || sub.Plan.Name != "") {
		dto.Plan = &AdminDirectoryPlan{
			Code: sub.Plan.Code,
			Name: sub.Plan.Name,
		}
	}
	return dto
}
