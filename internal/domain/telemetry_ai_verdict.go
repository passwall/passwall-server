package domain

import "time"

// TelemetryAIVerdict stores an AI-generated classification for a specific
// telemetry event group identified by its dedupe key (domain + page_path +
// event_name + error_code + flow_type + surface + succeeded).
//
// Once a verdict exists for a key, the analysis endpoint skips that group
// and returns the cached verdict instead of calling the LLM again.
type TelemetryAIVerdict struct {
	ID uint `gorm:"primary_key" json:"id"`

	CreatedAt time.Time `gorm:"index" json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Dedupe key — matches CompatTelemetrySummaryRow grouping.
	DomainETLD1 string `gorm:"type:varchar(255);not null;index:idx_verdict_key,unique" json:"domain_etld1"`
	PagePath    string `gorm:"type:varchar(512);not null;index:idx_verdict_key,unique" json:"page_path"`
	EventName   string `gorm:"type:varchar(80);not null;index:idx_verdict_key,unique" json:"event_name"`
	ErrorCode   string `gorm:"type:varchar(80);not null;index:idx_verdict_key,unique" json:"error_code"`
	FlowType    string `gorm:"type:varchar(32);not null;index:idx_verdict_key,unique" json:"flow_type"`
	Surface     string `gorm:"type:varchar(32);not null;index:idx_verdict_key,unique" json:"surface"`
	Succeeded   bool   `gorm:"not null;index:idx_verdict_key,unique" json:"succeeded"`

	// AI output
	Classification  string `gorm:"type:varchar(32);not null" json:"classification"` // bug, expected, needs_investigation, known_limitation
	Severity        string `gorm:"type:varchar(16);not null" json:"severity"`       // critical, high, medium, low, info
	Reasoning       string `gorm:"type:text" json:"reasoning"`                      // AI explanation
	SuggestedAction string `gorm:"type:text" json:"suggested_action"`               // What to do

	// Metadata
	Model      string `gorm:"type:varchar(64)" json:"model"`         // LLM model used
	EventCount int64  `gorm:"not null;default:0" json:"event_count"` // Count at time of analysis
}

func (TelemetryAIVerdict) TableName() string {
	return "telemetry_ai_verdicts"
}
