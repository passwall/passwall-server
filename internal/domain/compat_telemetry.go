package domain

import "time"

// CompatTelemetryEvent stores anonymized compatibility telemetry emitted by clients.
type CompatTelemetryEvent struct {
	ID uint `gorm:"primary_key" json:"id"`

	CreatedAt time.Time `gorm:"index" json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	UserID uint `gorm:"not null;index" json:"user_id"`

	DomainETLD1  string `gorm:"type:varchar(255);not null;index" json:"domain_etld1"`
	PagePath     string `gorm:"type:varchar(512);index" json:"page_path"`
	EventName    string `gorm:"type:varchar(80);not null;index" json:"event_name"`
	EventVersion int    `gorm:"not null;default:1" json:"event_version"`
	OccurredAt   string `gorm:"type:varchar(64)" json:"occurred_at"`

	FlowType string `gorm:"type:varchar(32);index" json:"flow_type"`
	Surface  string `gorm:"type:varchar(32);index" json:"surface"`

	Attempted bool `gorm:"not null;default:false" json:"attempted"`
	Succeeded bool `gorm:"not null;default:false" json:"succeeded"`

	ErrorCode string `gorm:"type:varchar(80);index" json:"error_code"`
	TimingMS  *int   `json:"timing_ms,omitempty"`

	PasswordFieldCount int `gorm:"not null;default:0" json:"password_field_count"`
	EmailFieldCount    int `gorm:"not null;default:0" json:"email_field_count"`
	UsernameFieldCount int `gorm:"not null;default:0" json:"username_field_count"`

	CaptchaDetected bool `gorm:"not null;default:false" json:"captcha_detected"`
	BotBlocked      bool `gorm:"not null;default:false" json:"bot_blocked"`

	StepIndex              int    `gorm:"not null;default:0" json:"step_index"`
	PrevStepHadIdentifier  bool   `gorm:"not null;default:false" json:"prev_step_had_identifier"`
	CurrStepHasPassword    bool   `gorm:"not null;default:false" json:"curr_step_has_password"`
	FieldVisibilityIssue   bool   `gorm:"not null;default:false" json:"field_visibility_issue"`
	FormMethod             string `gorm:"type:varchar(32)" json:"form_method"`
	DetectedFieldSignature string `gorm:"type:varchar(255)" json:"detected_field_signature"`

	ExtVersion     string `gorm:"type:varchar(64)" json:"ext_version"`
	Browser        string `gorm:"type:varchar(64)" json:"browser"`
	BrowserVersion string `gorm:"type:varchar(64)" json:"browser_version"`
	OS             string `gorm:"type:varchar(64)" json:"os"`

	SourceIP  string `gorm:"type:varchar(45)" json:"source_ip"`
	UserAgent string `gorm:"type:varchar(500)" json:"user_agent"`
}

func (CompatTelemetryEvent) TableName() string {
	return "compat_telemetry_events"
}

// CompatTelemetrySummaryRow is a deduplicated aggregate row for admin review.
type CompatTelemetrySummaryRow struct {
	DomainETLD1 string    `json:"domain_etld1"`
	PagePath    string    `json:"page_path"`
	EventName   string    `json:"event_name"`
	ErrorCode   string    `json:"error_code"`
	FlowType    string    `json:"flow_type"`
	Surface     string    `json:"surface"`
	Succeeded   bool      `json:"succeeded"`
	Count       int64     `json:"count"`
	FirstSeen   time.Time `json:"first_seen"`
	LastSeen    time.Time `json:"last_seen"`

	MaxStepIndex            int    `json:"max_step_index"`
	HasPrevStepIdentifier   bool   `json:"has_prev_step_identifier"`
	HasCurrStepPassword     bool   `json:"has_curr_step_password"`
	HasFieldVisibilityIssue bool   `json:"has_field_visibility_issue"`
	TopFormMethod           string `json:"top_form_method"`
	TopFieldSignature       string `json:"top_field_signature"`
}
