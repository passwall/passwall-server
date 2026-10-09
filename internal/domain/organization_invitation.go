package domain

import (
	"strings"
	"time"
)

// OrganizationInvitationStatus is the lifecycle state of an invitation to join
// an organization. Only "pending" invitations can be acted on.
type OrganizationInvitationStatus string

const (
	OrgInvitationPending  OrganizationInvitationStatus = "pending"
	OrgInvitationAccepted OrganizationInvitationStatus = "accepted"
	OrgInvitationDeclined OrganizationInvitationStatus = "declined"
	OrgInvitationRevoked  OrganizationInvitationStatus = "revoked"
	OrgInvitationExpired  OrganizationInvitationStatus = "expired"
)

// OrganizationInvitation is the single source of truth for an invitation to
// join an organization. No organization_users row exists until the invitee
// accepts; accepting creates the membership in one transaction.
//
// For an invitee who already has an account the inviter wraps the org key
// with the invitee's RSA public key (EncryptedOrgKey) so the membership works
// right after acceptance. Invitees without an account receive no key; after
// they sign up and accept, an admin confirms them (key exchange).
type OrganizationInvitation struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	OrganizationID uint                         `gorm:"not null;index" json:"organization_id"`
	Email          string                       `gorm:"type:varchar(255);not null" json:"email"`
	Role           OrganizationRole             `gorm:"type:varchar(20);not null" json:"role"`
	AccessAll      bool                         `gorm:"not null;default:false" json:"access_all"`
	Status         OrganizationInvitationStatus `gorm:"type:varchar(16);not null;default:'pending'" json:"status"`

	// EncryptedOrgKey is the org key wrapped with the invitee's RSA public key.
	EncryptedOrgKey *string `gorm:"type:text" json:"-"`

	InvitedByUserID uint       `gorm:"not null" json:"invited_by_user_id"`
	ExpiresAt       time.Time  `gorm:"not null" json:"expires_at"`
	LastSentAt      *time.Time `json:"last_sent_at,omitempty"`
	SendCount       int        `gorm:"not null;default:0" json:"send_count"`

	RespondedAt     *time.Time `json:"responded_at,omitempty"`
	AcceptedUserID  *uint      `json:"accepted_user_id,omitempty"`
	RevokedByUserID *uint      `json:"revoked_by_user_id,omitempty"`

	Organization *Organization `gorm:"foreignKey:OrganizationID" json:"-"`
	InvitedBy    *User         `gorm:"foreignKey:InvitedByUserID" json:"-"`
}

func (OrganizationInvitation) TableName() string { return "organization_invitations" }

// NormalizeInvitationEmail is the canonical form used for matching invitees.
func NormalizeInvitationEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// HasOrgKey reports whether the invitation already carries a wrapped org key.
func (i *OrganizationInvitation) HasOrgKey() bool {
	return i.EncryptedOrgKey != nil && *i.EncryptedOrgKey != ""
}

// EffectiveStatus reports "expired" for pending invitations past their expiry,
// even before the expiry worker has updated the row.
func (i *OrganizationInvitation) EffectiveStatus(now time.Time) OrganizationInvitationStatus {
	if i.Status == OrgInvitationPending && !now.Before(i.ExpiresAt) {
		return OrgInvitationExpired
	}
	return i.Status
}

// CreateOrgInvitationRequest invites someone to an organization.
type CreateOrgInvitationRequest struct {
	Email string           `json:"email" binding:"required,email"`
	Role  OrganizationRole `json:"role" binding:"required,oneof=owner admin manager member"`
	// EncryptedOrgKey is the org key wrapped with the invitee's RSA public key.
	// Sent when the invitee has an account with a key pair; ignored otherwise.
	EncryptedOrgKey string `json:"encrypted_org_key"`
	AccessAll       bool   `json:"access_all"`
}

// AcceptOrgInvitationRequest accepts a received invitation. EncryptedOrgKey is
// the org key re-wrapped with the invitee's user key; required when the
// invitation carried a key.
type AcceptOrgInvitationRequest struct {
	EncryptedOrgKey string `json:"encrypted_org_key"`
}

// InvitationPersonDTO identifies the inviter.
type InvitationPersonDTO struct {
	ID    uint   `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// OrgInvitationDTO is the admin view of an invitation.
type OrgInvitationDTO struct {
	ID         uint                         `json:"id"`
	Email      string                       `json:"email"`
	Role       OrganizationRole             `json:"role"`
	AccessAll  bool                         `json:"access_all"`
	Status     OrganizationInvitationStatus `json:"status"`
	InvitedBy  *InvitationPersonDTO         `json:"invited_by,omitempty"`
	CreatedAt  time.Time                    `json:"created_at"`
	ExpiresAt  time.Time                    `json:"expires_at"`
	LastSentAt *time.Time                   `json:"last_sent_at,omitempty"`
	SendCount  int                          `json:"send_count"`
	// RespondedAt is when the invitee accepted/declined or an admin revoked it.
	RespondedAt *time.Time `json:"responded_at,omitempty"`
	// RequiresConfirmation means no org key was shared with the invitation
	// (no account or no key pair yet), so an admin confirms the member after
	// acceptance.
	RequiresConfirmation bool `json:"requires_confirmation"`
}

// ReceivedInvitationDTO is the invitee's view of a pending invitation.
type ReceivedInvitationDTO struct {
	ID           uint                 `json:"id"`
	Organization InvitationOrgDTO     `json:"organization"`
	InvitedBy    *InvitationPersonDTO `json:"invited_by,omitempty"`
	Role         OrganizationRole     `json:"role"`
	CreatedAt    time.Time            `json:"created_at"`
	ExpiresAt    time.Time            `json:"expires_at"`
	// EncryptedOrgKey is wrapped with the invitee's RSA public key; the client
	// re-wraps it with the user key when accepting.
	EncryptedOrgKey *string `json:"encrypted_org_key,omitempty"`
	// RequiresAdminConfirmation is true when no key was shared: after
	// accepting, an admin must confirm the member before vault access.
	RequiresAdminConfirmation bool `json:"requires_admin_confirmation"`
}

// InvitationOrgDTO is the minimal organization info shown to an invitee.
type InvitationOrgDTO struct {
	ID       uint   `json:"id"`
	PublicID string `json:"public_id"`
	Name     string `json:"name"`
}

// ToOrgInvitationDTO converts for admin listings.
func ToOrgInvitationDTO(inv *OrganizationInvitation, now time.Time) *OrgInvitationDTO {
	dto := &OrgInvitationDTO{
		ID:                   inv.ID,
		Email:                inv.Email,
		Role:                 inv.Role,
		AccessAll:            inv.AccessAll,
		Status:               inv.EffectiveStatus(now),
		CreatedAt:            inv.CreatedAt,
		ExpiresAt:            inv.ExpiresAt,
		LastSentAt:           inv.LastSentAt,
		SendCount:            inv.SendCount,
		RespondedAt:          inv.RespondedAt,
		RequiresConfirmation: !inv.HasOrgKey(),
	}
	if inv.InvitedBy != nil {
		dto.InvitedBy = &InvitationPersonDTO{ID: inv.InvitedBy.ID, Name: inv.InvitedBy.Name, Email: inv.InvitedBy.Email}
	}
	return dto
}

// ToReceivedInvitationDTO converts for the invitee.
func ToReceivedInvitationDTO(inv *OrganizationInvitation) *ReceivedInvitationDTO {
	dto := &ReceivedInvitationDTO{
		ID:                        inv.ID,
		Role:                      inv.Role,
		CreatedAt:                 inv.CreatedAt,
		ExpiresAt:                 inv.ExpiresAt,
		RequiresAdminConfirmation: !inv.HasOrgKey(),
	}
	if inv.HasOrgKey() {
		dto.EncryptedOrgKey = inv.EncryptedOrgKey
	}
	if inv.Organization != nil {
		dto.Organization = InvitationOrgDTO{ID: inv.Organization.ID, PublicID: inv.Organization.PublicID, Name: inv.Organization.Name}
	} else {
		dto.Organization = InvitationOrgDTO{ID: inv.OrganizationID}
	}
	if inv.InvitedBy != nil {
		dto.InvitedBy = &InvitationPersonDTO{ID: inv.InvitedBy.ID, Name: inv.InvitedBy.Name, Email: inv.InvitedBy.Email}
	}
	return dto
}
