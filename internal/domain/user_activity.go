package domain

import (
	"time"
)

// ActivityType represents the type of user activity
type ActivityType string

const (
	ActivityTypeSignIn         ActivityType = "signin"
	ActivityTypeSignOut        ActivityType = "signout"
	ActivityTypePasswordChange ActivityType = "password_change"
	ActivityTypeEmailVerified  ActivityType = "email_verified"
	ActivityTypeAccountCreated ActivityType = "account_created"
	ActivityTypeVaultUnlock    ActivityType = "vault_unlock"
	ActivityTypeVaultLock      ActivityType = "vault_lock"
	ActivityTypeItemCreated    ActivityType = "item_created"
	ActivityTypeItemUpdated    ActivityType = "item_updated"
	ActivityTypeItemDeleted    ActivityType = "item_deleted"
	ActivityTypeFailedSignIn   ActivityType = "failed_signin"
)

// UserActivity represents user activity log for audit trail
type UserActivity struct {
	ID           uint         `gorm:"primary_key" json:"id"`
	UserID       uint         `gorm:"not null;index" json:"user_id"`
	ActivityType ActivityType `gorm:"type:varchar(50);not null;index" json:"activity_type"`
	IPAddress    string       `gorm:"type:varchar(45)" json:"ip_address"` // IPv4 or IPv6
	UserAgent    string       `gorm:"type:varchar(500)" json:"user_agent"`
	Details      string       `gorm:"type:text" json:"details,omitempty"` // JSON for additional info
	CreatedAt    time.Time    `gorm:"index" json:"created_at"`
}

// TableName specifies the table name
func (UserActivity) TableName() string {
	return "user_activities"
}

// UserActivityDTO for API responses
type UserActivityDTO struct {
	ID           uint         `json:"id"`
	ActivityType ActivityType `json:"activity_type"`
	IPAddress    string       `json:"ip_address"`
	UserAgent    string       `json:"user_agent"`
	Details      string       `json:"details,omitempty"`
	CreatedAt    time.Time    `json:"created_at"`
}

// CreateActivityRequest for logging activity
type CreateActivityRequest struct {
	UserID       uint
	ActivityType ActivityType
	IPAddress    string
	UserAgent    string
	Details      string
}

// ToUserActivityDTO converts UserActivity to DTO
func ToUserActivityDTO(activity *UserActivity) *UserActivityDTO {
	if activity == nil {
		return nil
	}

	return &UserActivityDTO{
		ID:           activity.ID,
		ActivityType: activity.ActivityType,
		IPAddress:    activity.IPAddress,
		UserAgent:    activity.UserAgent,
		Details:      activity.Details,
		CreatedAt:    activity.CreatedAt,
	}
}

// ToUserActivityDTOs converts multiple activities to DTOs
func ToUserActivityDTOs(activities []*UserActivity) []*UserActivityDTO {
	dtos := make([]*UserActivityDTO, len(activities))
	for i, activity := range activities {
		dtos[i] = ToUserActivityDTO(activity)
	}
	return dtos
}
