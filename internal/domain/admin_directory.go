package domain

import (
	"time"

	"github.com/google/uuid"
)

// AdminDirectorySchemaVersion is the contract version for the read-only admin directory API.
// Additive JSON fields do not change this value. Increment it only when a client would break.
const AdminDirectorySchemaVersion = 2

// AdminDirectorySubscriptionUnknown is returned when no subscription row is stored
// for the user's personal organization.
const AdminDirectorySubscriptionUnknown = "unknown"

// AdminDirectoryUser is the stable per-account record returned to service clients.
//
// New fields may be added. Clients must ignore unknown keys. This struct must never
// grow vault payloads, passwords, notes, cards, bank accounts, secrets, tokens, or keys.
// Counts and timestamps describe stored rows only. They do not include item contents,
// titles, usernames, URLs, device ids, or client names.
type AdminDirectoryUser struct {
	ID                uint                       `json:"id"`
	UUID              string                     `json:"uuid"`
	Email             string                     `json:"email"`
	EmailVerified     bool                       `json:"email_verified"`
	RegisteredAt      time.Time                  `json:"registered_at"`
	LastLoginAt       *time.Time                 `json:"last_login_at"`
	LastActivityAt    *time.Time                 `json:"last_activity_at"`
	Plan              *AdminDirectoryPlan        `json:"plan"`
	Subscription      AdminDirectorySubscription `json:"subscription"`
	OrganizationItems AdminDirectoryItemCounts   `json:"organization_items"`
	OrganizationCount int                        `json:"organization_count"`
	CollectionCount   int                        `json:"collection_count"`
	SignInCount       int                        `json:"signin_count"`
	ActivityCount     int                        `json:"activity_count"`
	DeviceCount       int                        `json:"device_count"`
	ClientCount       int                        `json:"client_count"`
}

// AdminDirectoryItemCounts is a count of stored items grouped by type.
//
// ItemCount and ByType are JSON null when that table cannot be counted.
// A table that exists and has no live rows is item_count 0 and by_type {}.
// ByType values are positive and sum to ItemCount. Keys are type names, never item data.
type AdminDirectoryItemCounts struct {
	ItemCount *int           `json:"item_count"`
	ByType    map[string]int `json:"by_type"`
}

// AdminDirectorySignals is count-only usage data loaded for one account.
// It is an input to NewAdminDirectoryUser and is not itself a response body.
//
// Organization item counts are always known: a nil map means the account created none.
// Numeric fields are known zeros when no matching row is stored.
type AdminDirectorySignals struct {
	LastActivityAt    *time.Time
	OrgItemsByType    map[ItemType]int
	OrganizationCount int
	CollectionCount   int
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
// signals carries count-only usage data.
func NewAdminDirectoryUser(user *User, lastLogin *time.Time, sub *Subscription, signals AdminDirectorySignals) AdminDirectoryUser {
	dto := AdminDirectoryUser{
		Subscription: AdminDirectorySubscription{Status: AdminDirectorySubscriptionUnknown},
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
	if signals.LastActivityAt != nil && !signals.LastActivityAt.IsZero() {
		active := signals.LastActivityAt.UTC()
		dto.LastActivityAt = &active
	}
	dto.OrganizationItems = directoryItemCounts(true, signals.OrgItemsByType)
	dto.OrganizationCount = nonNegative(signals.OrganizationCount)
	dto.CollectionCount = nonNegative(signals.CollectionCount)
	dto.SignInCount = nonNegative(signals.SignInCount)
	dto.ActivityCount = nonNegative(signals.ActivityCount)
	dto.DeviceCount = nonNegative(signals.DeviceCount)
	dto.ClientCount = nonNegative(signals.ClientCount)
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

// directoryItemCounts builds the public count object.
// Unknown item type numbers are summed under "unknown" so the total still matches.
func directoryItemCounts(known bool, counts map[ItemType]int) AdminDirectoryItemCounts {
	if !known {
		return AdminDirectoryItemCounts{}
	}
	byType := make(map[string]int)
	total := 0
	unknown := 0
	for itemType, count := range counts {
		if count <= 0 {
			continue
		}
		total += count
		key, ok := adminDirectoryItemTypeKey(itemType)
		if !ok {
			unknown += count
			continue
		}
		byType[key] += count
	}
	if unknown > 0 {
		byType["unknown"] = unknown
	}
	return AdminDirectoryItemCounts{
		ItemCount: &total,
		ByType:    byType,
	}
}

func nonNegative(n int) int {
	if n < 0 {
		return 0
	}
	return n
}

// adminDirectoryItemTypeKey returns the stable directory key for a stored item type.
func adminDirectoryItemTypeKey(itemType ItemType) (string, bool) {
	switch itemType {
	case ItemTypePassword:
		return "password", true
	case ItemTypeSecureNote:
		return "secure_note", true
	case ItemTypeCard:
		return "card", true
	case ItemTypeBankAccount:
		return "bank_account", true
	case ItemTypeEmail:
		return "email", true
	case ItemTypeServer:
		return "server", true
	case ItemTypeIdentity:
		return "identity", true
	case ItemTypeSSHKey:
		return "ssh_key", true
	case ItemTypeAddress:
		return "address", true
	case ItemTypePasskey:
		return "passkey", true
	case ItemTypeCustom:
		return "custom", true
	default:
		return "", false
	}
}
