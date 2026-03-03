package domain

import (
	"time"

	uuid "github.com/satori/go.uuid"
)

// EmergencyAccessStatus represents the status of an emergency access grant
type EmergencyAccessStatus string

const (
	EAStatusInvited           EmergencyAccessStatus = "invited"
	EAStatusAccepted          EmergencyAccessStatus = "accepted"
	EAStatusConfirmed         EmergencyAccessStatus = "confirmed"
	EAStatusRecoveryRequested EmergencyAccessStatus = "recovery_requested"
	EAStatusRecoveryApproved  EmergencyAccessStatus = "recovery_approved"
	EAStatusRecoveryRejected  EmergencyAccessStatus = "recovery_rejected"
	EAStatusRevoked           EmergencyAccessStatus = "revoked"
)

// EmergencyAccess represents a trust relationship for emergency vault access.
// The grantor allows the grantee to request view access to their vault.
// Zero-knowledge: the grantor's User Key is encrypted with the grantee's RSA public key.
type EmergencyAccess struct {
	ID        uint      `gorm:"primary_key" json:"id"`
	UUID      uuid.UUID `gorm:"type:uuid;not null;uniqueIndex" json:"uuid"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	GrantorID    uint   `json:"grantor_id" gorm:"not null;index;constraint:OnDelete:CASCADE"`
	GranteeID    *uint  `json:"grantee_id,omitempty" gorm:"index;constraint:OnDelete:SET NULL"`
	GranteeEmail string `json:"grantee_email" gorm:"type:varchar(255);not null"`

	Status EmergencyAccessStatus `json:"status" gorm:"type:varchar(30);not null;default:'invited';index"`

	// Grantor's UserKey encrypted with Grantee's RSA public key (set at confirm step)
	KeyEncrypted *string `json:"-" gorm:"type:text"`

	RecoveryInitAt    *time.Time `json:"recovery_init_at,omitempty"`
	RecoveryApproveAt *time.Time `json:"recovery_approve_at,omitempty"`
	LastNotifiedAt    *time.Time `json:"-"`

	// Associations
	Grantor *User `json:"grantor,omitempty" gorm:"foreignKey:GrantorID"`
	Grantee *User `json:"grantee,omitempty" gorm:"foreignKey:GranteeID"`
}

func (EmergencyAccess) TableName() string {
	return "emergency_accesses"
}

// EmergencyAccessDTO for API responses
type EmergencyAccessDTO struct {
	ID                uint                  `json:"id"`
	UUID              uuid.UUID             `json:"uuid"`
	GrantorID         uint                  `json:"grantor_id"`
	GrantorEmail      string                `json:"grantor_email,omitempty"`
	GrantorName       string                `json:"grantor_name,omitempty"`
	GranteeID         *uint                 `json:"grantee_id,omitempty"`
	GranteeEmail      string                `json:"grantee_email"`
	GranteeName       string                `json:"grantee_name,omitempty"`
	Status            EmergencyAccessStatus `json:"status"`
	CreatedAt         time.Time             `json:"created_at"`
	UpdatedAt         time.Time             `json:"updated_at"`
	RecoveryInitAt    *time.Time            `json:"recovery_init_at,omitempty"`
	RecoveryApproveAt *time.Time            `json:"recovery_approve_at,omitempty"`
}

func ToEmergencyAccessDTO(ea *EmergencyAccess) *EmergencyAccessDTO {
	if ea == nil {
		return nil
	}

	dto := &EmergencyAccessDTO{
		ID:                ea.ID,
		UUID:              ea.UUID,
		GrantorID:         ea.GrantorID,
		GranteeID:         ea.GranteeID,
		GranteeEmail:      ea.GranteeEmail,
		Status:            ea.Status,
		CreatedAt:         ea.CreatedAt,
		UpdatedAt:         ea.UpdatedAt,
		RecoveryInitAt:    ea.RecoveryInitAt,
		RecoveryApproveAt: ea.RecoveryApproveAt,
	}

	if ea.Grantor != nil {
		dto.GrantorEmail = ea.Grantor.Email
		dto.GrantorName = ea.Grantor.Name
	}
	if ea.Grantee != nil {
		dto.GranteeEmail = ea.Grantee.Email
		dto.GranteeName = ea.Grantee.Name
	}

	return dto
}

// CreateEmergencyAccessRequest for inviting a trusted contact
type CreateEmergencyAccessRequest struct {
	Email string `json:"email" validate:"required,email"`
}

// ConfirmEmergencyAccessRequest for confirming with key exchange
type ConfirmEmergencyAccessRequest struct {
	KeyEncrypted string `json:"key_encrypted" validate:"required"`
}
