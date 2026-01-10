package domain

import (
	"time"

	uuid "github.com/satori/go.uuid"
)

// SubscriptionState represents the state of a subscription
type SubscriptionState string

const (
	SubStateDraft     SubscriptionState = "draft"
	SubStateTrialing  SubscriptionState = "trialing"
	SubStateActive    SubscriptionState = "active"
	SubStatePastDue   SubscriptionState = "past_due"
	SubStateCanceled  SubscriptionState = "canceled"
	SubStateExpired   SubscriptionState = "expired"
)

// String returns the string representation of SubscriptionState
func (s SubscriptionState) String() string {
	return string(s)
}

// Subscription represents an organization's subscription to a plan
type Subscription struct {
	ID        uint      `gorm:"primary_key" json:"id"`
	UUID      uuid.UUID `gorm:"type:uuid;not null" json:"uuid"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	OrganizationID uint              `json:"organization_id" gorm:"not null;index;constraint:OnDelete:CASCADE"`
	PlanID         uint              `json:"plan_id" gorm:"not null;index"`
	State          SubscriptionState `json:"state" gorm:"type:varchar(20);not null;default:'draft'"`

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
	Organization *Organization `json:"organization,omitempty" gorm:"foreignKey:OrganizationID"`
	Plan         *Plan         `json:"plan,omitempty" gorm:"foreignKey:PlanID"`
}

// TableName specifies the table name
func (Subscription) TableName() string {
	return "subscriptions"
}

// IsActive checks if subscription allows full access
func (s *Subscription) IsActive() bool {
	return s.State == SubStateActive || s.State == SubStateTrialing || s.State == SubStatePastDue
}

// CanWrite checks if subscription allows write operations
func (s *Subscription) CanWrite() bool {
	return s.State != SubStateExpired
}

// IsInGracePeriod checks if subscription is in grace period after payment failure
func (s *Subscription) IsInGracePeriod() bool {
	return s.State == SubStatePastDue && s.GracePeriodEndsAt != nil && time.Now().Before(*s.GracePeriodEndsAt)
}

// IsCanceled checks if subscription is canceled
func (s *Subscription) IsCanceled() bool {
	return s.State == SubStateCanceled
}

// IsExpired checks if subscription has expired
func (s *Subscription) IsExpired() bool {
	return s.State == SubStateExpired
}

// ShouldExpire checks if subscription should be expired (grace period ended or cancel period ended)
func (s *Subscription) ShouldExpire() bool {
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

// SubscriptionDTO for API responses
type SubscriptionDTO struct {
	ID                   uint              `json:"id"`
	UUID                 uuid.UUID         `json:"uuid"`
	OrganizationID       uint              `json:"organization_id"`
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

// CreateSubscriptionRequest for API requests
type CreateSubscriptionRequest struct {
	PlanCode string `json:"plan_code" validate:"required"`
}

// UpdateSubscriptionRequest for API requests
type UpdateSubscriptionRequest struct {
	PlanCode string `json:"plan_code" validate:"required"`
}

// CancelSubscriptionRequest for API requests
type CancelSubscriptionRequest struct {
	Immediate bool `json:"immediate"` // Cancel immediately or at period end
}

// ToSubscriptionDTO converts Subscription to DTO
func ToSubscriptionDTO(s *Subscription) *SubscriptionDTO {
	if s == nil {
		return nil
	}

	dto := &SubscriptionDTO{
		ID:                   s.ID,
		UUID:                 s.UUID,
		OrganizationID:       s.OrganizationID,
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

