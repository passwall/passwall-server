package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/passwall/passwall-server/internal/config"
	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
)

// TelemetryAnalysisResult is the AI-generated classification for a group of telemetry events.
type TelemetryAnalysisResult struct {
	Domain          string `json:"domain"`
	PagePath        string `json:"page_path"`
	ErrorCode       string `json:"error_code"`
	FlowType        string `json:"flow_type"`
	Surface         string `json:"surface"`
	EventCount      int64  `json:"event_count"`
	Classification  string `json:"classification"`   // "bug", "expected", "needs_investigation", "known_limitation"
	Severity        string `json:"severity"`         // "critical", "high", "medium", "low", "info"
	Reasoning       string `json:"reasoning"`        // AI explanation of why this classification was made
	SuggestedAction string `json:"suggested_action"` // What the team should do
	Cached          bool   `json:"cached"`           // true if returned from stored verdict (no LLM call)
}

// TelemetryAnalysisResponse wraps the full AI analysis output.
type TelemetryAnalysisResponse struct {
	AnalyzedAt  time.Time                 `json:"analyzed_at"`
	Model       string                    `json:"model"`
	TotalEvents int                       `json:"total_events"`
	NewAnalyzed int                       `json:"new_analyzed"` // Groups sent to AI in this call
	CachedCount int                       `json:"cached_count"` // Groups returned from cached verdicts
	Results     []TelemetryAnalysisResult `json:"results"`
	Summary     string                    `json:"summary"` // High-level executive summary
}

// AITelemetryAnalysisService analyses compatibility telemetry using an LLM.
type AITelemetryAnalysisService interface {
	// Analyze fetches deduplicated telemetry, skips already-verdicted groups,
	// sends only new groups to the LLM, persists verdicts, and returns merged results.
	Analyze(ctx context.Context, filter repository.CompatTelemetryListFilter) (*TelemetryAnalysisResponse, error)
	// ListVerdicts returns all stored AI verdicts.
	ListVerdicts(ctx context.Context, limit int) ([]*domain.TelemetryAIVerdict, error)
	// ResetVerdicts deletes all stored verdicts so the next Analyze re-evaluates everything.
	ResetVerdicts(ctx context.Context) (int64, error)
}

type aiTelemetryAnalysisService struct {
	aiConfig      *config.AIConfig
	telemetryRepo repository.CompatTelemetryRepository
	verdictRepo   repository.TelemetryAIVerdictRepository
	httpClient    *http.Client
	logger        Logger
}

// NewAITelemetryAnalysisService creates a new AI telemetry analysis service.
func NewAITelemetryAnalysisService(
	aiConfig *config.AIConfig,
	telemetryRepo repository.CompatTelemetryRepository,
	verdictRepo repository.TelemetryAIVerdictRepository,
	logger Logger,
) AITelemetryAnalysisService {
	timeout := 60
	if aiConfig.Timeout > 0 {
		timeout = aiConfig.Timeout
	}
	return &aiTelemetryAnalysisService{
		aiConfig:      aiConfig,
		telemetryRepo: telemetryRepo,
		verdictRepo:   verdictRepo,
		httpClient: &http.Client{
			Timeout: time.Duration(timeout) * time.Second,
		},
		logger: logger,
	}
}

// verdictKeyFromSummary builds a lookup key string for a summary row.
func verdictKeyFromSummary(row *domain.CompatTelemetrySummaryRow) string {
	return strings.Join([]string{
		row.DomainETLD1, row.PagePath, row.EventName,
		row.ErrorCode, row.FlowType, row.Surface,
		fmt.Sprintf("%v", row.Succeeded),
	}, "|")
}

// verdictKeyFromVerdict builds the same lookup key from a stored verdict.
func verdictKeyFromVerdict(v *domain.TelemetryAIVerdict) string {
	return strings.Join([]string{
		v.DomainETLD1, v.PagePath, v.EventName,
		v.ErrorCode, v.FlowType, v.Surface,
		fmt.Sprintf("%v", v.Succeeded),
	}, "|")
}

func (s *aiTelemetryAnalysisService) Analyze(
	ctx context.Context,
	filter repository.CompatTelemetryListFilter,
) (*TelemetryAnalysisResponse, error) {
	if !s.aiConfig.Enabled {
		return nil, fmt.Errorf("AI analysis is not enabled")
	}
	if s.aiConfig.APIKey == "" {
		return nil, fmt.Errorf("AI API key is not configured")
	}

	// 1. Fetch deduplicated summary data.
	summaryFilter := filter
	if summaryFilter.Limit == 0 {
		summaryFilter.Limit = 200
	}
	summaries, total, err := s.telemetryRepo.ListSummary(ctx, summaryFilter)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch telemetry summary: %w", err)
	}

	if len(summaries) == 0 {
		return &TelemetryAnalysisResponse{
			AnalyzedAt:  time.Now(),
			Model:       s.aiConfig.Model,
			TotalEvents: 0,
			Results:     []TelemetryAnalysisResult{},
			Summary:     "No telemetry events found for the given filters.",
		}, nil
	}

	// 2. Build lookup keys for all summary rows and fetch existing verdicts.
	keys := make([]repository.VerdictKey, 0, len(summaries))
	for _, row := range summaries {
		keys = append(keys, repository.VerdictKey{
			DomainETLD1: row.DomainETLD1,
			PagePath:    row.PagePath,
			EventName:   row.EventName,
			ErrorCode:   row.ErrorCode,
			FlowType:    row.FlowType,
			Surface:     row.Surface,
			Succeeded:   row.Succeeded,
		})
	}

	existingVerdicts, err := s.verdictRepo.FindByKeys(ctx, keys)
	if err != nil {
		s.logger.Warn("failed to load existing verdicts, will re-analyze all", "error", err)
		existingVerdicts = nil
	}

	// Build a set of already-verdicted keys.
	verdictMap := make(map[string]*domain.TelemetryAIVerdict, len(existingVerdicts))
	for _, v := range existingVerdicts {
		verdictMap[verdictKeyFromVerdict(v)] = v
	}

	// 3. Split summaries into cached (already have a verdict) and new (need AI).
	var cachedResults []TelemetryAnalysisResult
	var newSummaries []*domain.CompatTelemetrySummaryRow

	for _, row := range summaries {
		key := verdictKeyFromSummary(row)
		if v, ok := verdictMap[key]; ok {
			// Already analyzed — return cached verdict with current event count.
			cachedResults = append(cachedResults, TelemetryAnalysisResult{
				Domain:          v.DomainETLD1,
				PagePath:        v.PagePath,
				ErrorCode:       v.ErrorCode,
				FlowType:        v.FlowType,
				Surface:         v.Surface,
				EventCount:      row.Count, // Use current count from live data.
				Classification:  v.Classification,
				Severity:        v.Severity,
				Reasoning:       v.Reasoning,
				SuggestedAction: v.SuggestedAction,
				Cached:          true,
			})
		} else {
			newSummaries = append(newSummaries, row)
		}
	}

	// 4. If there are no new groups, return cached results immediately (no LLM call).
	if len(newSummaries) == 0 {
		return &TelemetryAnalysisResponse{
			AnalyzedAt:  time.Now(),
			Model:       s.aiConfig.Model,
			TotalEvents: int(total),
			NewAnalyzed: 0,
			CachedCount: len(cachedResults),
			Results:     cachedResults,
			Summary:     "All event groups have been previously analyzed. No new LLM call needed.",
		}, nil
	}

	// 5. Call LLM only for new (un-analyzed) groups.
	s.logger.Info("AI telemetry analysis",
		"new_groups", len(newSummaries),
		"cached_groups", len(cachedResults),
		"total_summary_rows", len(summaries),
	)

	prompt := s.buildPrompt(newSummaries, total)
	raw, err := s.callLLM(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("AI analysis failed: %w", err)
	}

	newResults, summary, err := s.parseResponse(raw)
	if err != nil {
		// Parsing failed — return raw as summary with cached results.
		return &TelemetryAnalysisResponse{
			AnalyzedAt:  time.Now(),
			Model:       s.aiConfig.Model,
			TotalEvents: int(total),
			NewAnalyzed: len(newSummaries),
			CachedCount: len(cachedResults),
			Results:     cachedResults,
			Summary:     raw,
		}, nil
	}

	// 6. Persist new verdicts to the database.
	verdicts := make([]*domain.TelemetryAIVerdict, 0, len(newResults))
	// Build a lookup from error_code+domain+path to actual summary row for metadata.
	summaryLookup := make(map[string]*domain.CompatTelemetrySummaryRow, len(newSummaries))
	for _, row := range newSummaries {
		key := row.DomainETLD1 + "|" + row.PagePath + "|" + row.ErrorCode + "|" + row.FlowType + "|" + row.Surface
		summaryLookup[key] = row
	}

	for _, r := range newResults {
		v := &domain.TelemetryAIVerdict{
			DomainETLD1:     r.Domain,
			PagePath:        r.PagePath,
			EventName:       "detect_result",
			ErrorCode:       r.ErrorCode,
			FlowType:        r.FlowType,
			Surface:         r.Surface,
			Succeeded:       false,
			Classification:  r.Classification,
			Severity:        r.Severity,
			Reasoning:       r.Reasoning,
			SuggestedAction: r.SuggestedAction,
			Model:           s.aiConfig.Model,
			EventCount:      r.EventCount,
		}
		// Match to original summary to get correct event_name/succeeded.
		lookupKey := r.Domain + "|" + r.PagePath + "|" + r.ErrorCode + "|" + r.FlowType + "|" + r.Surface
		if row, ok := summaryLookup[lookupKey]; ok {
			v.EventName = row.EventName
			v.Succeeded = row.Succeeded
		}
		verdicts = append(verdicts, v)
	}

	if err := s.verdictRepo.UpsertBatch(ctx, verdicts); err != nil {
		s.logger.Error("failed to persist AI verdicts", "count", len(verdicts), "error", err)
	} else {
		s.logger.Info("AI verdicts persisted", "count", len(verdicts))
	}

	// 7. Merge cached + new results.
	allResults := make([]TelemetryAnalysisResult, 0, len(cachedResults)+len(newResults))
	allResults = append(allResults, newResults...)
	allResults = append(allResults, cachedResults...)

	return &TelemetryAnalysisResponse{
		AnalyzedAt:  time.Now(),
		Model:       s.aiConfig.Model,
		TotalEvents: int(total),
		NewAnalyzed: len(newResults),
		CachedCount: len(cachedResults),
		Results:     allResults,
		Summary:     summary,
	}, nil
}

func (s *aiTelemetryAnalysisService) ListVerdicts(ctx context.Context, limit int) ([]*domain.TelemetryAIVerdict, error) {
	return s.verdictRepo.ListAll(ctx, limit)
}

func (s *aiTelemetryAnalysisService) ResetVerdicts(ctx context.Context) (int64, error) {
	return s.verdictRepo.DeleteAll(ctx)
}

func (s *aiTelemetryAnalysisService) buildPrompt(
	summaries []*domain.CompatTelemetrySummaryRow,
	total int64,
) string {
	var sb strings.Builder
	sb.WriteString(`You are a senior QA engineer at PassWall, an open-source password manager. You are analyzing browser extension compatibility telemetry data to determine which events are bugs, which are expected behavior, and which need investigation.

## Context — PassWall Extension Compatibility Telemetry

The browser extension runs a content script on every page. It scans for login/signup forms and attempts autofill. When something goes wrong (or succeeds), it emits a telemetry event.

### Error Codes Reference:
- **FORM_DYNAMIC_NOT_READY**: SPA page — form inputs not rendered when extension scanned. Extension will rescan via MutationObserver. Common on React/Vue/Angular SPAs. Usually resolves itself.
- **NO_PASSWORD_FIELD**: No visible <input type="password"> found. Expected on multi-step login (Google, Microsoft) where the first step only shows email. Also expected on non-login pages.
- **NO_USERNAME_CANDIDATE**: Multi-step login detection couldn't find username/email field. May indicate unusual field naming or obfuscation.
- **IFRAME_CROSS_ORIGIN_BLOCKED**: Password field is inside a cross-origin iframe. Browser security prevents content script access. Known limitation.
- **FRAME_DETACHED**: Iframe removed from DOM before extension could interact.
- **SHADOW_SCAN_LIMIT_REACHED**: Shadow DOM traversal hit depth/count limit.
- **CAPTCHA_OR_BOT_CHALLENGE**: CAPTCHA or bot protection detected on the page.
- **SUBMIT_NOT_OBSERVED**: Form was filled but submission event was not detected.

### Surface Types:
- **form_based**: Traditional HTML <form> with inputs.
- **formless**: SPA-style inputs without a wrapping <form> element.
- **multi_step**: Multi-step wizard login (email first, then password on next step).
- **iframe**: Form is inside an iframe.
- **shadow_dom**: Form uses Shadow DOM encapsulation.

### Flow Types:
- **login**: Sign-in form.
- **signup**: Registration form.
- **change_password**: Password change form.
- **unknown**: Could not determine the form's purpose.

### Key Rules for Classification:
1. NO_PASSWORD_FIELD on the ROOT page (page_path = domain only, no path) → almost always **expected** (homepage, not a login page).
2. NO_PASSWORD_FIELD followed by multi_step success on the same domain → **expected** (multi-step login working correctly).
3. FORM_DYNAMIC_NOT_READY with low count (< 5) → likely **expected** (SPA timing, rescan catches it).
4. FORM_DYNAMIC_NOT_READY with high count on a specific page_path → likely a **bug** (extension consistently fails there).
5. IFRAME_CROSS_ORIGIN_BLOCKED → always **known_limitation** (browser security, not fixable in extension code alone).
6. High failure count on major/popular websites → higher severity.
7. error_code empty + succeeded=true → **expected** (successful operation, no issue).

## Your Task

Analyze each row of telemetry summary data below. For each row, determine:
1. **classification**: One of "bug", "expected", "needs_investigation", "known_limitation"
2. **severity**: One of "critical", "high", "medium", "low", "info"
3. **reasoning**: A brief explanation (1-2 sentences) of why you chose this classification.
4. **suggested_action**: What the PassWall team should do about this (1 sentence).

Then provide a high-level **summary** paragraph covering the overall health and top priorities.

## Telemetry Summary Data

`)

	sb.WriteString(fmt.Sprintf("New event groups to analyze: %d (from total %d)\n\n", total, len(summaries)))
	sb.WriteString("| # | domain | page_path | error_code | flow_type | surface | succeeded | count | first_seen | last_seen |\n")
	sb.WriteString("|---|--------|-----------|------------|-----------|---------|-----------|-------|------------|----------|\n")

	for i, row := range summaries {
		errorCode := row.ErrorCode
		if errorCode == "" {
			errorCode = "(none)"
		}
		sb.WriteString(fmt.Sprintf("| %d | %s | %s | %s | %s | %s | %v | %d | %s | %s |\n",
			i+1,
			row.DomainETLD1,
			row.PagePath,
			errorCode,
			row.FlowType,
			row.Surface,
			row.Succeeded,
			row.Count,
			row.FirstSeen.Format("2006-01-02"),
			row.LastSeen.Format("2006-01-02"),
		))
	}

	sb.WriteString(`
## Required Output Format

Respond ONLY with valid JSON matching this exact structure:

{
  "results": [
    {
      "domain": "example.com",
      "page_path": "example.com/login",
      "error_code": "NO_PASSWORD_FIELD",
      "flow_type": "login",
      "surface": "unknown",
      "event_count": 42,
      "classification": "expected",
      "severity": "info",
      "reasoning": "No password field on multi-step login first step, expected for Google-style login.",
      "suggested_action": "No action needed, multi-step detection handles this."
    }
  ],
  "summary": "Overall health assessment paragraph here."
}

Do NOT include any text outside the JSON object. Do NOT use markdown code fences.
`)

	return sb.String()
}

// openAIChatRequest is the request body for the OpenAI-compatible chat completions API.
type openAIChatRequest struct {
	Model       string              `json:"model"`
	Messages    []openAIChatMessage `json:"messages"`
	Temperature float64             `json:"temperature"`
	MaxTokens   int                 `json:"max_tokens,omitempty"`
}

type openAIChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIChatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (s *aiTelemetryAnalysisService) callLLM(ctx context.Context, prompt string) (string, error) {
	baseURL := strings.TrimRight(s.aiConfig.BaseURL, "/")
	endpoint := baseURL + "/chat/completions"

	reqBody := openAIChatRequest{
		Model: s.aiConfig.Model,
		Messages: []openAIChatMessage{
			{
				Role:    "system",
				Content: "You are a senior QA engineer analyzing browser extension compatibility telemetry. You output ONLY valid JSON, no markdown, no commentary.",
			},
			{
				Role:    "user",
				Content: prompt,
			},
		},
		Temperature: 0.1,
		MaxTokens:   4096,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.aiConfig.APIKey)

	// Anthropic-specific header (ignored by OpenAI)
	if strings.Contains(strings.ToLower(s.aiConfig.Provider), "anthropic") {
		req.Header.Set("x-api-key", s.aiConfig.APIKey)
		req.Header.Set("anthropic-version", "2023-06-01")
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("LLM request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("LLM returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var chatResp openAIChatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return "", fmt.Errorf("failed to parse LLM response: %w", err)
	}

	if chatResp.Error != nil && chatResp.Error.Message != "" {
		return "", fmt.Errorf("LLM error: %s", chatResp.Error.Message)
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("LLM returned no choices")
	}

	return strings.TrimSpace(chatResp.Choices[0].Message.Content), nil
}

func (s *aiTelemetryAnalysisService) parseResponse(raw string) ([]TelemetryAnalysisResult, string, error) {
	// Strip potential markdown code fences
	cleaned := raw
	if idx := strings.Index(cleaned, "{"); idx >= 0 {
		cleaned = cleaned[idx:]
	}
	if idx := strings.LastIndex(cleaned, "}"); idx >= 0 {
		cleaned = cleaned[:idx+1]
	}

	var parsed struct {
		Results []TelemetryAnalysisResult `json:"results"`
		Summary string                    `json:"summary"`
	}
	if err := json.Unmarshal([]byte(cleaned), &parsed); err != nil {
		return nil, "", fmt.Errorf("failed to parse AI JSON: %w", err)
	}

	// Validate classifications
	validClassifications := map[string]bool{
		"bug": true, "expected": true, "needs_investigation": true, "known_limitation": true,
	}
	validSeverities := map[string]bool{
		"critical": true, "high": true, "medium": true, "low": true, "info": true,
	}
	for i := range parsed.Results {
		if !validClassifications[parsed.Results[i].Classification] {
			parsed.Results[i].Classification = "needs_investigation"
		}
		if !validSeverities[parsed.Results[i].Severity] {
			parsed.Results[i].Severity = "medium"
		}
	}

	return parsed.Results, parsed.Summary, nil
}
