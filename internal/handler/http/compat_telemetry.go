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

type CompatTelemetryHandler struct {
	service service.CompatTelemetryService
}

func NewCompatTelemetryHandler(service service.CompatTelemetryService) *CompatTelemetryHandler {
	return &CompatTelemetryHandler{service: service}
}

// Ingest handles compatibility telemetry ingestion from authenticated clients.
// POST /api/telemetry/compat
func (h *CompatTelemetryHandler) Ingest(c *gin.Context) {
	userID, err := GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req service.CompatTelemetryBatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	ingested, err := h.service.IngestBatch(
		c.Request.Context(),
		userID,
		GetIPAddress(c),
		GetUserAgent(c),
		&req,
	)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"ingested": ingested,
	})
}

type CompatTelemetryAdminListResponse struct {
	Items    interface{} `json:"items"`
	Total    int64       `json:"total"`
	Filtered int64       `json:"filtered"`
}

// ListAdmin returns compatibility telemetry events for admin dashboards.
// GET /api/admin/telemetry/compat
func (h *CompatTelemetryHandler) ListAdmin(c *gin.Context) {
	limit := clampTelemetryQueryInt(c.Query("limit"), 50, 1, 500)
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

	filter := repository.CompatTelemetryListFilter{
		Search:    strings.TrimSpace(c.Query("q")),
		Domain:    strings.ToLower(strings.TrimSpace(c.Query("domain"))),
		EventName: strings.TrimSpace(c.Query("event_name")),
		FlowType:  strings.TrimSpace(c.Query("flow_type")),
		Surface:   strings.TrimSpace(c.Query("surface")),
		ErrorCode: strings.TrimSpace(c.Query("error_code")),
		Succeeded: succeeded,
		Order:     order,
		Limit:     limit,
		Offset:    offset,
	}
	filter.CreatedAfter = parseRetentionQuery(c.Query("retention_days"), 90)

	items, total, filtered, err := h.service.ListForAdmin(c.Request.Context(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list telemetry events"})
		return
	}

	c.JSON(http.StatusOK, CompatTelemetryAdminListResponse{
		Items:    items,
		Total:    total,
		Filtered: filtered,
	})
}

// ListSummaryAdmin returns deduplicated summary of compatibility telemetry for admin review.
// GET /api/admin/telemetry/compat/summary
func (h *CompatTelemetryHandler) ListSummaryAdmin(c *gin.Context) {
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
	filter := repository.CompatTelemetryListFilter{
		Search:       strings.TrimSpace(c.Query("q")),
		Domain:       strings.ToLower(strings.TrimSpace(c.Query("domain"))),
		EventName:    strings.TrimSpace(c.Query("event_name")),
		FlowType:     strings.TrimSpace(c.Query("flow_type")),
		Surface:      strings.TrimSpace(c.Query("surface")),
		ErrorCode:    strings.TrimSpace(c.Query("error_code")),
		Succeeded:    succeeded,
		Order:        order,
		Limit:        limit,
		Offset:       offset,
		CreatedAfter: parseRetentionQuery(c.Query("retention_days"), 90),
	}
	items, total, err := h.service.ListSummaryForAdmin(c.Request.Context(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list telemetry summary"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"items": items,
		"total": total,
	})
}

// CleanupAdmin deletes compatibility telemetry older than retention_days (default 90).
// POST /api/admin/telemetry/compat/cleanup
func (h *CompatTelemetryHandler) CleanupAdmin(c *gin.Context) {
	days := clampTelemetryQueryInt(c.Query("retention_days"), 90, 1, 3650)
	before := time.Now().AddDate(0, 0, -days)
	deleted, err := h.service.DeleteOlderThan(c.Request.Context(), before)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to cleanup telemetry"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": deleted, "older_than_days": days})
}

func parseRetentionQuery(raw string, defaultDays int) *time.Time {
	d := defaultDays
	if strings.TrimSpace(raw) != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(raw)); err == nil && n > 0 {
			d = n
		}
	}
	t := time.Now().AddDate(0, 0, -d)
	return &t
}

func clampTelemetryQueryInt(raw string, fallback int, min int, max int) int {
	value := fallback
	if strings.TrimSpace(raw) != "" {
		parsed, err := strconv.Atoi(raw)
		if err == nil {
			value = parsed
		}
	}
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}
