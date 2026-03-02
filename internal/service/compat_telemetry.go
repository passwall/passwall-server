package service

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
)

var compatDomainPattern = regexp.MustCompile(`^[a-z0-9.-]+\.[a-z]{2,}$`)

// CompatTelemetryBatchRequest is a batched ingest payload from clients.
type CompatTelemetryBatchRequest struct {
	Events []CompatTelemetryEventPayload `json:"events"`
}

// CompatTelemetryEventPayload is a single compatibility event.
type CompatTelemetryEventPayload struct {
	EventName    string `json:"event_name"`
	EventVersion int    `json:"event_version"`
	OccurredAt   string `json:"occurred_at"`

	DomainETLD1 string `json:"domain_etld1"`
	FlowType    string `json:"flow_type"`
	Surface     string `json:"surface"`
	Attempted   bool   `json:"attempted"`
	Succeeded   bool   `json:"succeeded"`
	ErrorCode   string `json:"error_code"`
	TimingMS    *int   `json:"timing_ms"`

	PasswordFieldCount int  `json:"password_field_count"`
	EmailFieldCount    int  `json:"email_field_count"`
	UsernameFieldCount int  `json:"username_field_count"`
	CaptchaDetected    bool `json:"captcha_detected"`
	BotBlocked         bool `json:"bot_blocked"`

	ExtVersion     string `json:"ext_version"`
	Browser        string `json:"browser"`
	BrowserVersion string `json:"browser_version"`
	OS             string `json:"os"`
}

type compatTelemetryService struct {
	repo   repository.CompatTelemetryRepository
	logger Logger
}

// NewCompatTelemetryService creates a new compatibility telemetry service.
func NewCompatTelemetryService(repo repository.CompatTelemetryRepository, logger Logger) CompatTelemetryService {
	return &compatTelemetryService{
		repo:   repo,
		logger: logger,
	}
}

func (s *compatTelemetryService) IngestBatch(
	ctx context.Context,
	userID uint,
	sourceIP string,
	userAgent string,
	req *CompatTelemetryBatchRequest,
) (int, error) {
	if req == nil || len(req.Events) == 0 {
		return 0, fmt.Errorf("events are required")
	}

	events := make([]*domain.CompatTelemetryEvent, 0, len(req.Events))

	for _, payload := range req.Events {
		normalized, err := normalizeCompatPayload(payload)
		if err != nil {
			return 0, err
		}

		events = append(events, &domain.CompatTelemetryEvent{
			UserID: userID,

			DomainETLD1:  normalized.DomainETLD1,
			EventName:    normalized.EventName,
			EventVersion: normalized.EventVersion,
			OccurredAt:   normalized.OccurredAt,

			FlowType: normalized.FlowType,
			Surface:  normalized.Surface,

			Attempted: normalized.Attempted,
			Succeeded: normalized.Succeeded,
			ErrorCode: normalized.ErrorCode,
			TimingMS:  normalized.TimingMS,

			PasswordFieldCount: normalized.PasswordFieldCount,
			EmailFieldCount:    normalized.EmailFieldCount,
			UsernameFieldCount: normalized.UsernameFieldCount,
			CaptchaDetected:    normalized.CaptchaDetected,
			BotBlocked:         normalized.BotBlocked,

			ExtVersion:     normalized.ExtVersion,
			Browser:        normalized.Browser,
			BrowserVersion: normalized.BrowserVersion,
			OS:             normalized.OS,

			SourceIP:  sourceIP,
			UserAgent: userAgent,
		})
	}

	if err := s.repo.CreateBatch(ctx, events); err != nil {
		s.logger.Error("failed to ingest compatibility telemetry", "count", len(events), "error", err)
		return 0, err
	}

	return len(events), nil
}

func (s *compatTelemetryService) ListForAdmin(
	ctx context.Context,
	filter repository.CompatTelemetryListFilter,
) ([]*domain.CompatTelemetryEvent, int64, int64, error) {
	return s.repo.List(ctx, filter)
}

func normalizeCompatPayload(payload CompatTelemetryEventPayload) (*CompatTelemetryEventPayload, error) {
	eventName := strings.TrimSpace(payload.EventName)
	if eventName == "" {
		return nil, fmt.Errorf("event_name is required")
	}
	if len(eventName) > 80 {
		eventName = eventName[:80]
	}

	eventVersion := payload.EventVersion
	if eventVersion <= 0 {
		eventVersion = 1
	}

	normalizedDomain := strings.ToLower(strings.TrimSpace(payload.DomainETLD1))
	if normalizedDomain == "" {
		return nil, fmt.Errorf("domain_etld1 is required")
	}
	if len(normalizedDomain) > 255 || !compatDomainPattern.MatchString(normalizedDomain) {
		return nil, fmt.Errorf("domain_etld1 is invalid")
	}

	occurredAt := strings.TrimSpace(payload.OccurredAt)
	if occurredAt != "" {
		if _, err := time.Parse(time.RFC3339, occurredAt); err != nil {
			occurredAt = ""
		}
	}

	flowType := truncateLower(payload.FlowType, 32)
	surface := truncateLower(payload.Surface, 32)
	errorCode := truncateUpper(payload.ErrorCode, 80)

	extVersion := truncate(payload.ExtVersion, 64)
	browser := truncate(payload.Browser, 64)
	browserVersion := truncate(payload.BrowserVersion, 64)
	osName := truncate(payload.OS, 64)

	passwordCount := clampInt(payload.PasswordFieldCount, 0, 20)
	emailCount := clampInt(payload.EmailFieldCount, 0, 20)
	usernameCount := clampInt(payload.UsernameFieldCount, 0, 20)

	var timingMS *int
	if payload.TimingMS != nil {
		clamped := clampInt(*payload.TimingMS, 0, 120000)
		timingMS = &clamped
	}

	return &CompatTelemetryEventPayload{
		EventName:    eventName,
		EventVersion: eventVersion,
		OccurredAt:   occurredAt,
		DomainETLD1:  normalizedDomain,
		FlowType:     flowType,
		Surface:      surface,
		Attempted:    payload.Attempted,
		Succeeded:    payload.Succeeded,
		ErrorCode:    errorCode,
		TimingMS:     timingMS,

		PasswordFieldCount: passwordCount,
		EmailFieldCount:    emailCount,
		UsernameFieldCount: usernameCount,
		CaptchaDetected:    payload.CaptchaDetected,
		BotBlocked:         payload.BotBlocked,

		ExtVersion:     extVersion,
		Browser:        browser,
		BrowserVersion: browserVersion,
		OS:             osName,
	}, nil
}

func truncate(value string, maxLen int) string {
	value = strings.TrimSpace(value)
	if len(value) <= maxLen {
		return value
	}

	return value[:maxLen]
}

func truncateLower(value string, maxLen int) string {
	return strings.ToLower(truncate(value, maxLen))
}

func truncateUpper(value string, maxLen int) string {
	return strings.ToUpper(truncate(value, maxLen))
}

func clampInt(value int, min int, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}

	return value
}
