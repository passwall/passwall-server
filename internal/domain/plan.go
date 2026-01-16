package domain

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	uuid "github.com/satori/go.uuid"
)

// PlanFeatures represents feature flags for a plan
type PlanFeatures struct {
	Items           *int `json:"items"`            // Max items (null = unlimited)
	Sharing         bool `json:"sharing"`          // Item sharing enabled
	Teams           bool `json:"teams"`            // Team management enabled
	Audit           bool `json:"audit"`            // Audit logs enabled
	SSO             bool `json:"sso"`              // Single Sign-On enabled
	APIAccess       bool `json:"api_access"`       // API access enabled
	PrioritySupport bool `json:"priority_support"` // Priority support enabled
}

// Scan implements sql.Scanner for PlanFeatures (JSONB)
func (f *PlanFeatures) Scan(value interface{}) error {
	if value == nil {
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("failed to scan PlanFeatures: expected []byte, got %T", value)
	}

	return json.Unmarshal(bytes, f)
}

// Value implements driver.Valuer for PlanFeatures (JSONB)
func (f PlanFeatures) Value() (driver.Value, error) {
	return json.Marshal(f)
}

// Plan represents a subscription plan
type Plan struct {
	ID        uint      `gorm:"primary_key" json:"id"`
	UUID      uuid.UUID `gorm:"type:uuid;not null" json:"uuid"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	Code         string       `json:"code" gorm:"type:varchar(50);uniqueIndex;not null"`
	Name         string       `json:"name" gorm:"type:varchar(100);not null"`
	BillingCycle BillingCycle `json:"billing_cycle" gorm:"type:varchar(20);not null"`
	PriceCents   int          `json:"price_cents" gorm:"not null"`
	Currency     string       `json:"currency" gorm:"type:varchar(3);not null;default:'USD'"`
	TrialDays    int          `json:"trial_days" gorm:"not null;default:0"`

	// Limits (null = unlimited)
	MaxUsers       *int `json:"max_users,omitempty"`
	MaxCollections *int `json:"max_collections,omitempty"`
	MaxItems       *int `json:"max_items,omitempty"`

	// Feature flags
	Features PlanFeatures `json:"features" gorm:"type:jsonb;not null"`

	// Status
	IsActive bool `json:"is_active" gorm:"default:true"`

	// Stripe integration
	StripeProductID *string `json:"stripe_product_id,omitempty" gorm:"type:varchar(255)"`
	StripePriceID   *string `json:"stripe_price_id,omitempty" gorm:"type:varchar(255)"`
}

// TableName specifies the table name
func (Plan) TableName() string {
	return "plans"
}

// IsFree checks if plan is free
func (p *Plan) IsFree() bool {
	return p.PriceCents == 0
}

// HasTrial checks if plan has trial period
func (p *Plan) HasTrial() bool {
	return p.TrialDays > 0
}

// IsUnlimitedUsers checks if plan allows unlimited users
func (p *Plan) IsUnlimitedUsers() bool {
	return p.MaxUsers == nil
}

// IsUnlimitedCollections checks if plan allows unlimited collections
func (p *Plan) IsUnlimitedCollections() bool {
	return p.MaxCollections == nil
}

// IsUnlimitedItems checks if plan allows unlimited items
func (p *Plan) IsUnlimitedItems() bool {
	return p.MaxItems == nil
}

// GetPriceDisplay returns formatted price for display (e.g., "$5.99")
func (p *Plan) GetPriceDisplay() string {
	if p.PriceCents == 0 {
		return "Free"
	}
	dollars := float64(p.PriceCents) / 100.0
	return fmt.Sprintf("$%.2f", dollars)
}

// PlanDTO for API responses
type PlanDTO struct {
	ID             uint         `json:"id"`
	UUID           uuid.UUID    `json:"uuid"`
	Code           string       `json:"code"`
	Name           string       `json:"name"`
	BillingCycle   BillingCycle `json:"billing_cycle"`
	PriceCents     int          `json:"price_cents"`
	PriceDisplay   string       `json:"price_display"`
	Currency       string       `json:"currency"`
	TrialDays      int          `json:"trial_days"`
	MaxUsers       *int         `json:"max_users,omitempty"`
	MaxCollections *int         `json:"max_collections,omitempty"`
	MaxItems       *int         `json:"max_items,omitempty"`
	Features       PlanFeatures `json:"features"`
	IsActive       bool         `json:"is_active"`
	CreatedAt      time.Time    `json:"created_at"`
	UpdatedAt      time.Time    `json:"updated_at"`
}

// ToPlanDTO converts Plan to DTO
func ToPlanDTO(p *Plan) *PlanDTO {
	if p == nil {
		return nil
	}

	return &PlanDTO{
		ID:             p.ID,
		UUID:           p.UUID,
		Code:           p.Code,
		Name:           p.Name,
		BillingCycle:   p.BillingCycle,
		PriceCents:     p.PriceCents,
		PriceDisplay:   p.GetPriceDisplay(),
		Currency:       p.Currency,
		TrialDays:      p.TrialDays,
		MaxUsers:       p.MaxUsers,
		MaxCollections: p.MaxCollections,
		MaxItems:       p.MaxItems,
		Features:       p.Features,
		IsActive:       p.IsActive,
		CreatedAt:      p.CreatedAt,
		UpdatedAt:      p.UpdatedAt,
	}
}
