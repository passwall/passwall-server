package domain

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

// MonitoredEmail represents an email address being monitored for data breaches.
type MonitoredEmail struct {
	ID             uint       `gorm:"primaryKey" json:"id"`
	OrganizationID uint       `gorm:"not null;index" json:"organization_id"`
	Email          string     `gorm:"type:varchar(320);not null" json:"email"`
	LastCheckedAt  *time.Time `json:"last_checked_at"`
	BreachCount    int        `gorm:"default:0" json:"breach_count"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`

	BreachRecords []BreachRecord `gorm:"foreignKey:MonitoredEmailID" json:"breach_records,omitempty"`
}

func (MonitoredEmail) TableName() string {
	return "monitored_emails"
}

// StringSliceJSON stores a JSON array of strings (e.g. data classes).
type StringSliceJSON []string

func (s *StringSliceJSON) Scan(value interface{}) error {
	if value == nil {
		*s = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("StringSliceJSON.Scan: expected []byte, got %T", value)
	}
	return json.Unmarshal(bytes, s)
}

func (s StringSliceJSON) Value() (driver.Value, error) {
	if s == nil {
		return "[]", nil
	}
	return json.Marshal(s)
}

// BreachRecord represents a single data breach found for a monitored email.
type BreachRecord struct {
	ID               uint            `gorm:"primaryKey" json:"id"`
	MonitoredEmailID uint            `gorm:"not null;index;constraint:OnDelete:CASCADE" json:"monitored_email_id"`
	BreachName       string          `gorm:"type:varchar(255);not null" json:"breach_name"`
	BreachDomain     string          `gorm:"type:varchar(255)" json:"breach_domain"`
	BreachDate       string          `gorm:"type:varchar(10)" json:"breach_date"`
	AddedDate        string          `gorm:"type:varchar(30)" json:"added_date"`
	DataClasses      StringSliceJSON `gorm:"type:jsonb;default:'[]'" json:"data_classes"`
	Description      string          `gorm:"type:text" json:"description"`
	LogoPath         string          `gorm:"type:varchar(512)" json:"logo_path"`
	PwnCount         int             `json:"pwn_count"`
	IsVerified       bool            `gorm:"default:false" json:"is_verified"`
	IsSensitive      bool            `gorm:"default:false" json:"is_sensitive"`
	IsDismissed      bool            `gorm:"default:false" json:"is_dismissed"`
	DiscoveredAt     time.Time       `json:"discovered_at"`
	CreatedAt        time.Time       `json:"created_at"`
}

func (BreachRecord) TableName() string {
	return "breach_records"
}

// ── DTOs ────────────────────────────────────────────────────

// MonitoredEmailDTO is the API response for a monitored email.
type MonitoredEmailDTO struct {
	ID            uint              `json:"id"`
	Email         string            `json:"email"`
	LastCheckedAt *time.Time        `json:"last_checked_at"`
	BreachCount   int               `json:"breach_count"`
	CreatedAt     time.Time         `json:"created_at"`
	Breaches      []BreachRecordDTO `json:"breaches,omitempty"`
}

// BreachRecordDTO is the API response for a breach record.
type BreachRecordDTO struct {
	ID           uint            `json:"id"`
	BreachName   string          `json:"breach_name"`
	BreachDomain string          `json:"breach_domain"`
	BreachDate   string          `json:"breach_date"`
	AddedDate    string          `json:"added_date"`
	DataClasses  StringSliceJSON `json:"data_classes"`
	Description  string          `json:"description"`
	LogoPath     string          `json:"logo_path"`
	PwnCount     int             `json:"pwn_count"`
	IsVerified   bool            `json:"is_verified"`
	IsSensitive  bool            `json:"is_sensitive"`
	IsDismissed  bool            `json:"is_dismissed"`
	DiscoveredAt time.Time       `json:"discovered_at"`
}

// BreachMonitorSummaryDTO is returned by the summary endpoint.
type BreachMonitorSummaryDTO struct {
	MonitoredEmails int        `json:"monitored_emails"`
	TotalBreaches   int        `json:"total_breaches"`
	ActiveBreaches  int        `json:"active_breaches"`
	LastCheckedAt   *time.Time `json:"last_checked_at"`
}

// ── Converters ──────────────────────────────────────────────

func ToMonitoredEmailDTO(m *MonitoredEmail) *MonitoredEmailDTO {
	dto := &MonitoredEmailDTO{
		ID:            m.ID,
		Email:         m.Email,
		LastCheckedAt: m.LastCheckedAt,
		BreachCount:   m.BreachCount,
		CreatedAt:     m.CreatedAt,
	}
	for _, b := range m.BreachRecords {
		dto.Breaches = append(dto.Breaches, *ToBreachRecordDTO(&b))
	}
	return dto
}

func ToBreachRecordDTO(b *BreachRecord) *BreachRecordDTO {
	return &BreachRecordDTO{
		ID:           b.ID,
		BreachName:   b.BreachName,
		BreachDomain: b.BreachDomain,
		BreachDate:   b.BreachDate,
		AddedDate:    b.AddedDate,
		DataClasses:  b.DataClasses,
		Description:  b.Description,
		LogoPath:     b.LogoPath,
		PwnCount:     b.PwnCount,
		IsVerified:   b.IsVerified,
		IsSensitive:  b.IsSensitive,
		IsDismissed:  b.IsDismissed,
		DiscoveredAt: b.DiscoveredAt,
	}
}
