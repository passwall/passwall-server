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

	// Admin / Audit activities (admin-only visibility in UI)
	ActivityTypeAdminUserCreated ActivityType = "admin_user_created"
	ActivityTypeAdminUserUpdated ActivityType = "admin_user_updated"
	ActivityTypeAdminUserDeleted ActivityType = "admin_user_deleted"
	
	// Billing & Subscription Activities
	ActivityTypeCheckoutCreated           ActivityType = "checkout_created"
	ActivityTypeSubscriptionCreated       ActivityType = "subscription_created"
	ActivityTypeSubscriptionUpdated       ActivityType = "subscription_updated"
	ActivityTypeSubscriptionCanceled      ActivityType = "subscription_canceled"
	ActivityTypeSubscriptionReactivated   ActivityType = "subscription_reactivated"
	ActivityTypeInvoicePaid               ActivityType = "invoice_paid"
	ActivityTypeInvoicePaymentFailed      ActivityType = "invoice_payment_failed"
	ActivityTypeOrganizationUpgraded      ActivityType = "organization_upgraded"
	ActivityTypeOrganizationDowngraded    ActivityType = "organization_downgraded"
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
	UserID       uint         `json:"user_id"`
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
		UserID:       activity.UserID,
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
