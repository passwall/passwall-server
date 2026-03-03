package service

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
)

const compatTelemetryDedupeWindow = 7 * 24 * time.Hour

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
	PagePath    string `json:"page_path"`
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
			PagePath:     normalized.PagePath,
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

	existingSet, err := s.repo.ListExistingCompatKeys(ctx, time.Now().Add(-compatTelemetryDedupeWindow))
	if err != nil {
		s.logger.Warn("failed to load existing telemetry keys for dedupe", "error", err)
		existingSet = nil
	}

	keySet := make(map[string]struct{})
	for _, k := range existingSet {
		keySet[compatDedupeKeyString(k.DomainETLD1, k.PagePath, k.EventName, k.ErrorCode, k.FlowType, k.Surface, k.Succeeded)] = struct{}{}
	}

	filtered := make([]*domain.CompatTelemetryEvent, 0, len(events))
	for _, e := range events {
		key := compatDedupeKeyString(e.DomainETLD1, e.PagePath, e.EventName, normErrorCodeForDedupe(e.ErrorCode), e.FlowType, e.Surface, e.Succeeded)
		if _, exists := keySet[key]; exists {
			continue
		}
		keySet[key] = struct{}{}
		filtered = append(filtered, e)
	}

	if len(filtered) == 0 {
		return 0, nil
	}

	if err := s.repo.CreateBatch(ctx, filtered); err != nil {
		s.logger.Error("failed to ingest compatibility telemetry", "count", len(filtered), "error", err)
		return 0, err
	}

	if skipped := len(events) - len(filtered); skipped > 0 {
		s.logger.Debug("compat telemetry dedupe skipped", "skipped", skipped, "ingested", len(filtered))
	}

	return len(filtered), nil
}

func normErrorCodeForDedupe(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "none"
	}
	return s
}

func compatDedupeKeyString(domainETLD1, pagePath, eventName, errorCode, flowType, surface string, succeeded bool) string {
	return strings.Join([]string{
		domainETLD1, pagePath, eventName, normErrorCodeForDedupe(errorCode), flowType, surface,
		strconv.FormatBool(succeeded),
	}, "|")
}

func (s *compatTelemetryService) ListForAdmin(
	ctx context.Context,
	filter repository.CompatTelemetryListFilter,
) ([]*domain.CompatTelemetryEvent, int64, int64, error) {
	return s.repo.List(ctx, filter)
}

func (s *compatTelemetryService) ListSummaryForAdmin(
	ctx context.Context,
	filter repository.CompatTelemetryListFilter,
) ([]*domain.CompatTelemetrySummaryRow, int64, error) {
	return s.repo.ListSummary(ctx, filter)
}

func (s *compatTelemetryService) DeleteOlderThan(ctx context.Context, before time.Time) (int64, error) {
	return s.repo.DeleteOlderThan(ctx, before)
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

	pagePath := truncateLower(payload.PagePath, 512)
	if pagePath == "" {
		pagePath = normalizedDomain
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
		PagePath:     pagePath,
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
