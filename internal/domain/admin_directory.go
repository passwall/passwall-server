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
type AdminDirectoryUser struct {
	ID           uint                       `json:"id"`
	UUID         string                     `json:"uuid"`
	Email        string                     `json:"email"`
	RegisteredAt time.Time                  `json:"registered_at"`
	LastLoginAt  *time.Time                 `json:"last_login_at"`
	Plan         *AdminDirectoryPlan        `json:"plan"`
	Subscription AdminDirectorySubscription `json:"subscription"`
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
func NewAdminDirectoryUser(user *User, lastLogin *time.Time, sub *Subscription) AdminDirectoryUser {
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
