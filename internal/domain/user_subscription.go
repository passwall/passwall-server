package domain

import (
	"time"

	uuid "github.com/satori/go.uuid"
)

// UserSubscription represents a user's personal subscription to a plan (e.g., Pro)
// This is separate from organization subscriptions - every user can have their own subscription.
type UserSubscription struct {
	ID        uint      `gorm:"primary_key" json:"id"`
	UUID      uuid.UUID `gorm:"type:uuid;not null" json:"uuid"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	UserID uint              `json:"user_id" gorm:"not null;uniqueIndex;constraint:OnDelete:CASCADE"`
	PlanID uint              `json:"plan_id" gorm:"not null;index"`
	State  SubscriptionState `json:"state" gorm:"type:varchar(20);not null;default:'draft'"`

	// Lifecycle timestamps
	StartedAt         *time.Time `json:"started_at,omitempty"`
	RenewAt           *time.Time `json:"renew_at,omitempty"`
	CancelAt          *time.Time `json:"cancel_at,omitempty"`
	EndedAt           *time.Time `json:"ended_at,omitempty"`
	GracePeriodEndsAt *time.Time `json:"grace_period_ends_at,omitempty"`
	TrialEndsAt       *time.Time `json:"trial_ends_at,omitempty"`

	// Stripe integration
	StripeSubscriptionID *string `json:"stripe_subscription_id,omitempty" gorm:"type:varchar(255);uniqueIndex"`

	// Associations
	User *User `json:"user,omitempty" gorm:"foreignKey:UserID"`
	Plan *Plan `json:"plan,omitempty" gorm:"foreignKey:PlanID"`
}

// TableName specifies the table name
func (UserSubscription) TableName() string {
	return "user_subscriptions"
}

// IsActive checks if subscription allows full access
func (s *UserSubscription) IsActive() bool {
	return s.State == SubStateActive || s.State == SubStateTrialing || s.State == SubStatePastDue
}

// CanWrite checks if subscription allows write operations
func (s *UserSubscription) CanWrite() bool {
	return s.State != SubStateExpired
}

// IsInGracePeriod checks if subscription is in grace period after payment failure
func (s *UserSubscription) IsInGracePeriod() bool {
	return s.State == SubStatePastDue && s.GracePeriodEndsAt != nil && time.Now().Before(*s.GracePeriodEndsAt)
}

// IsCanceled checks if subscription is canceled
func (s *UserSubscription) IsCanceled() bool {
	return s.State == SubStateCanceled
}

// IsExpired checks if subscription has expired
func (s *UserSubscription) IsExpired() bool {
	return s.State == SubStateExpired
}

// ShouldExpire checks if subscription should be expired
func (s *UserSubscription) ShouldExpire() bool {
	now := time.Now()

	// Past due with expired grace period
	if s.State == SubStatePastDue && s.GracePeriodEndsAt != nil && now.After(*s.GracePeriodEndsAt) {
		return true
	}

	// Canceled with expired period end
	if s.State == SubStateCanceled && s.RenewAt != nil && now.After(*s.RenewAt) {
		return true
	}

	return false
}

// UserSubscriptionDTO for API responses
type UserSubscriptionDTO struct {
	ID                   uint              `json:"id"`
	UUID                 uuid.UUID         `json:"uuid"`
	UserID               uint              `json:"user_id"`
	Plan                 *PlanDTO          `json:"plan,omitempty"`
	State                SubscriptionState `json:"state"`
	StartedAt            *time.Time        `json:"started_at,omitempty"`
	RenewAt              *time.Time        `json:"renew_at,omitempty"`
	CancelAt             *time.Time        `json:"cancel_at,omitempty"`
	EndedAt              *time.Time        `json:"ended_at,omitempty"`
	GracePeriodEndsAt    *time.Time        `json:"grace_period_ends_at,omitempty"`
	TrialEndsAt          *time.Time        `json:"trial_ends_at,omitempty"`
	StripeSubscriptionID *string           `json:"stripe_subscription_id,omitempty"`
	CreatedAt            time.Time         `json:"created_at"`
	UpdatedAt            time.Time         `json:"updated_at"`
}

// ToUserSubscriptionDTO converts UserSubscription to DTO
func ToUserSubscriptionDTO(s *UserSubscription) *UserSubscriptionDTO {
	if s == nil {
		return nil
	}

	dto := &UserSubscriptionDTO{
		ID:                   s.ID,
		UUID:                 s.UUID,
		UserID:               s.UserID,
		State:                s.State,
		StartedAt:            s.StartedAt,
		RenewAt:              s.RenewAt,
		CancelAt:             s.CancelAt,
		EndedAt:              s.EndedAt,
		GracePeriodEndsAt:    s.GracePeriodEndsAt,
		TrialEndsAt:          s.TrialEndsAt,
		StripeSubscriptionID: s.StripeSubscriptionID,
		CreatedAt:            s.CreatedAt,
		UpdatedAt:            s.UpdatedAt,
	}

	// Add plan if loaded
	if s.Plan != nil {
		dto.Plan = ToPlanDTO(s.Plan)
	}

	return dto
}

// UserBillingInfo represents billing information for a user
type UserBillingInfo struct {
	UserID       uint                 `json:"user_id"`
	Email        string               `json:"email"`
	Name         string               `json:"name"`
	IsPro        bool                 `json:"is_pro"` // true if user has active Pro subscription
	Subscription *UserSubscriptionDTO `json:"subscription,omitempty"`
	CurrentPlan  string               `json:"current_plan"` // "free" or plan code like "pro-monthly"
	CurrentItems int                  `json:"current_items"`
	Invoices     []*InvoiceDTO        `json:"invoices,omitempty"`

	// Payment provider information
	Provider          PaymentProvider `json:"provider"`                     // "stripe", "revenuecat", "manual", "none"
	Store             string          `json:"store,omitempty"`              // "APP_STORE", "PLAY_STORE", etc. (only for revenuecat)
	StoreDisplayName  string          `json:"store_display_name,omitempty"` // "Apple App Store", "Google Play Store", etc.
	ManagedExternally bool            `json:"managed_externally"`           // true if subscription can only be canceled from external store
	CanCancel         bool            `json:"can_cancel"`                   // true if subscription can be canceled from our API
	CanUpgrade        bool            `json:"can_upgrade"`                  // true if plan can be changed from our API
}
