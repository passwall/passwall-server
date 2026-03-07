package http

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/passwall/passwall-server/internal/repository"
	"github.com/passwall/passwall-server/internal/service"
)

// AITelemetryHandler handles AI-powered telemetry analysis endpoints.
type AITelemetryHandler struct {
	analysisService service.AITelemetryAnalysisService
}

// NewAITelemetryHandler creates a new AI telemetry analysis handler.
func NewAITelemetryHandler(analysisService service.AITelemetryAnalysisService) *AITelemetryHandler {
	return &AITelemetryHandler{analysisService: analysisService}
}

// Analyze invokes the AI to classify compatibility telemetry events.
// GET /api/admin/telemetry/compat/analyze
func (h *AITelemetryHandler) Analyze(c *gin.Context) {
	limit := clampTelemetryQueryInt(c.Query("limit"), 100, 1, 500)
	offset := clampTelemetryQueryInt(c.Query("offset"), 0, 0, 10_000_000)
	order := strings.ToLower(strings.TrimSpace(c.DefaultQuery("order", "desc")))
	if order != "asc" && order != "desc" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid order (asc|desc expected)"})
		return
	}

	var succeeded *bool
	if raw := strings.TrimSpace(c.Query("succeeded")); raw != "" {
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid succeeded (true|false expected)"})
			return
		}
		succeeded = &parsed
	}

	retentionDays := 90
	if raw := strings.TrimSpace(c.Query("retention_days")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			retentionDays = n
		}
	}
	createdAfter := time.Now().AddDate(0, 0, -retentionDays)

	filter := repository.CompatTelemetryListFilter{
		Search:       strings.TrimSpace(c.Query("q")),
		Domain:       strings.ToLower(strings.TrimSpace(c.Query("domain"))),
		PagePath:     strings.TrimSpace(c.Query("page_path")),
		EventName:    strings.TrimSpace(c.Query("event_name")),
		FlowType:     strings.TrimSpace(c.Query("flow_type")),
		Surface:      strings.TrimSpace(c.Query("surface")),
		ErrorCode:    strings.TrimSpace(c.Query("error_code")),
		Succeeded:    succeeded,
		Order:        order,
		Limit:        limit,
		Offset:       offset,
		CreatedAfter: &createdAfter,
	}

	result, err := h.analysisService.Analyze(c.Request.Context(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// ResetVerdicts clears all stored AI verdicts, forcing re-analysis on next call.
// DELETE /api/admin/telemetry/compat/analyze/verdicts
func (h *AITelemetryHandler) ResetVerdicts(c *gin.Context) {
	deleted, err := h.analysisService.ResetVerdicts(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "All AI verdicts have been deleted. Next analysis will re-evaluate all event groups.",
		"deleted": deleted,
	})
}

// ListVerdicts returns all stored AI verdicts.
// GET /api/admin/telemetry/compat/analyze/verdicts
func (h *AITelemetryHandler) ListVerdicts(c *gin.Context) {
	limit := clampTelemetryQueryInt(c.Query("limit"), 200, 1, 1000)

	verdicts, err := h.analysisService.ListVerdicts(c.Request.Context(), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"verdicts": verdicts,
		"count":    len(verdicts),
	})
}
