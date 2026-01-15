package http

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"html"
	"net/http"
	"net/mail"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/passwall/passwall-server/internal/email"
)

type AdminBulkEmailHandler struct {
	emailSender email.Sender
	logger      email.Logger

	mu   sync.RWMutex
	jobs map[string]*bulkEmailJob
}

type bulkEmailJobState string

const (
	bulkEmailJobQueued   bulkEmailJobState = "queued"
	bulkEmailJobRunning  bulkEmailJobState = "running"
	bulkEmailJobFinished bulkEmailJobState = "finished"
	bulkEmailJobFailed   bulkEmailJobState = "failed"
)

type bulkEmailFailure struct {
	Email string `json:"email"`
	Error string `json:"error"`
}

type bulkEmailJob struct {
	ID        string           `json:"job_id"`
	State     bulkEmailJobState `json:"state"`
	CreatedAt time.Time        `json:"created_at"`
	StartedAt *time.Time       `json:"started_at,omitempty"`
	EndedAt   *time.Time       `json:"ended_at,omitempty"`

	Total  int `json:"total"`
	Sent   int `json:"sent"`
	Failed int `json:"failed"`

	Failures []bulkEmailFailure `json:"failures"`

	Subject string `json:"subject"`
}

type AdminBulkEmailCreateRequest struct {
	Recipients []string `json:"recipients" binding:"required,min=1,max=1500"`
	Subject    string   `json:"subject" binding:"required,min=1,max=200"`
	Message    string   `json:"message" binding:"required,min=1,max=20000"`
	IsHTML     *bool    `json:"is_html"`
}

type AdminBulkEmailCreateResponse struct {
	JobID string `json:"job_id"`
	Total int    `json:"total"`
}

func NewAdminBulkEmailHandler(emailSender email.Sender, logger email.Logger) *AdminBulkEmailHandler {
	return &AdminBulkEmailHandler{
		emailSender: emailSender,
		logger:      logger,
		jobs:        make(map[string]*bulkEmailJob),
	}
}

func (h *AdminBulkEmailHandler) CreateJob(c *gin.Context) {
	var req AdminBulkEmailCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid request",
			"details": err.Error(),
		})
		return
	}

	subject := strings.TrimSpace(req.Subject)
	if subject == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "subject is required"})
		return
	}

	message := strings.TrimSpace(req.Message)
	if message == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "message is required"})
		return
	}

	recipients, invalid := normalizeAndValidateEmails(req.Recipients)
	if len(invalid) > 0 {
		// Return a bounded sample of invalid emails to avoid huge payloads.
		sample := invalid
		if len(sample) > 50 {
			sample = sample[:50]
		}
		c.JSON(http.StatusBadRequest, gin.H{
			"error":         "invalid recipients",
			"invalid_count": len(invalid),
			"invalid":       sample,
		})
		return
	}

	if len(recipients) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "at least one recipient is required"})
		return
	}
	if len(recipients) > 1500 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "too many recipients (max 1500)"})
		return
	}

	jobID := newJobID()
	now := time.Now().UTC()

	job := &bulkEmailJob{
		ID:        jobID,
		State:     bulkEmailJobQueued,
		CreatedAt: now,
		Total:     len(recipients),
		Failures:  make([]bulkEmailFailure, 0),
		Subject:   subject,
	}

	h.mu.Lock()
	h.jobs[jobID] = job
	h.mu.Unlock()

	isHTML := false
	if req.IsHTML != nil {
		isHTML = *req.IsHTML
	} else {
		isHTML = detectLikelyHTML(message)
	}

	// Start async worker (avoid request timeout).
	go h.runJob(context.Background(), jobID, recipients, subject, message, isHTML)

	h.logger.Info("admin bulk email job created",
		"job_id", jobID,
		"recipient_count", len(recipients),
	)

	c.JSON(http.StatusOK, AdminBulkEmailCreateResponse{
		JobID: jobID,
		Total: len(recipients),
	})
}

func (h *AdminBulkEmailHandler) GetJob(c *gin.Context) {
	jobID := c.Param("jobId")
	if jobID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "jobId is required"})
		return
	}

	h.mu.RLock()
	job, ok := h.jobs[jobID]
	h.mu.RUnlock()
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "job not found"})
		return
	}

	c.JSON(http.StatusOK, job)
}

func (h *AdminBulkEmailHandler) runJob(
	ctx context.Context,
	jobID string,
	recipients []string,
	subject string,
	message string,
	isHTML bool,
) {
	// Mark job as running
	startedAt := time.Now().UTC()
	h.mu.Lock()
	job, ok := h.jobs[jobID]
	if !ok {
		h.mu.Unlock()
		return
	}
	job.State = bulkEmailJobRunning
	job.StartedAt = &startedAt
	h.mu.Unlock()

	const batchSize = 10
	const batchDelay = 1100 * time.Millisecond

	body := buildAdminBroadcastBody(message, isHTML)

	for i := 0; i < len(recipients); i += batchSize {
		if ctx.Err() != nil {
			break
		}

		end := i + batchSize
		if end > len(recipients) {
			end = len(recipients)
		}

		batch := recipients[i:end]
		for _, to := range batch {
			if ctx.Err() != nil {
				break
			}

			err := h.sendWithRetry(ctx, to, subject, body)
			h.mu.Lock()
			// job pointer still valid; stored in map
			if err != nil {
				job.Failed++
				job.Failures = append(job.Failures, bulkEmailFailure{
					Email: to,
					Error: err.Error(),
				})
			} else {
				job.Sent++
			}
			h.mu.Unlock()
		}

		// Gentle throttle between batches to reduce Gmail API quota/rate surprises.
		time.Sleep(batchDelay)
	}

	endedAt := time.Now().UTC()
	h.mu.Lock()
	defer h.mu.Unlock()
	job, ok = h.jobs[jobID]
	if !ok {
		return
	}
	job.EndedAt = &endedAt
	if job.Failed > 0 && job.Sent == 0 {
		job.State = bulkEmailJobFailed
	} else {
		job.State = bulkEmailJobFinished
	}

	h.logger.Info("admin bulk email job finished",
		"job_id", jobID,
		"total", job.Total,
		"sent", job.Sent,
		"failed", job.Failed,
	)
}

func (h *AdminBulkEmailHandler) sendWithRetry(ctx context.Context, to, subject, body string) error {
	const maxAttempts = 3
	backoff := 700 * time.Millisecond

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		err := h.emailSender.Send(ctx, &email.EmailMessage{
			To:      to,
			From:    "hello@passwall.io",
			Subject: subject,
			Body:    body,
		})
		if err == nil {
			return nil
		}

		// Best-effort detection: if it's likely a rate limit, back off and retry.
		lower := strings.ToLower(err.Error())
		isRate := strings.Contains(lower, "rate") || strings.Contains(lower, "429") || strings.Contains(lower, "quota")
		if attempt < maxAttempts && isRate {
			time.Sleep(backoff)
			backoff *= 2
			continue
		}

		return err
	}
	return nil
}

func normalizeAndValidateEmails(input []string) ([]string, []string) {
	seen := make(map[string]struct{}, len(input))
	out := make([]string, 0, len(input))
	invalid := make([]string, 0)

	for _, raw := range input {
		s := strings.TrimSpace(raw)
		if s == "" {
			continue
		}
		// Normalize
		s = strings.ToLower(s)

		if _, err := mail.ParseAddress(s); err != nil {
			invalid = append(invalid, s)
			continue
		}

		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}

	return out, invalid
}

func buildAdminBroadcastBody(message string, isHTML bool) string {
	msg := strings.TrimSpace(message)
	if isHTML {
		lower := strings.ToLower(msg)
		if strings.Contains(lower, "<html") || strings.Contains(lower, "<!doctype") {
			return msg
		}
		// Treat as HTML fragment and wrap into Passwall shell.
		return fmt.Sprintf(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8"/>
  <meta name="viewport" content="width=device-width, initial-scale=1"/>
  <title>Passwall</title>
</head>
<body style="margin:0;padding:0;background:#f6f7fb;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif;">
  <div style="max-width:640px;margin:0 auto;padding:28px 16px;">
    <div style="background:#ffffff;border:1px solid #e5e7eb;border-radius:12px;overflow:hidden;">
      <div style="padding:18px 22px;border-bottom:1px solid #eef2f7;">
        <div style="font-size:18px;font-weight:700;letter-spacing:-0.02em;color:#111827;">
          Passwall
        </div>
      </div>
      <div style="padding:22px;color:#111827;line-height:1.6;font-size:14px;">
        %s
      </div>
      <div style="padding:14px 22px;border-top:1px solid #eef2f7;color:#6b7280;font-size:12px;">
        Sent by Passwall
      </div>
    </div>
  </div>
</body>
</html>`, msg)
	}

	escaped := html.EscapeString(msg)
	escaped = strings.ReplaceAll(escaped, "\n", "<br/>")

	// Minimal, safe HTML template (no user-provided HTML).
	return fmt.Sprintf(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8"/>
  <meta name="viewport" content="width=device-width, initial-scale=1"/>
  <title>Passwall</title>
</head>
<body style="margin:0;padding:0;background:#f6f7fb;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif;">
  <div style="max-width:640px;margin:0 auto;padding:28px 16px;">
    <div style="background:#ffffff;border:1px solid #e5e7eb;border-radius:12px;overflow:hidden;">
      <div style="padding:18px 22px;border-bottom:1px solid #eef2f7;">
        <div style="font-size:18px;font-weight:700;letter-spacing:-0.02em;color:#111827;">
          Passwall
        </div>
      </div>
      <div style="padding:22px;color:#111827;line-height:1.6;font-size:14px;">
        %s
      </div>
      <div style="padding:14px 22px;border-top:1px solid #eef2f7;color:#6b7280;font-size:12px;">
        Sent by Passwall
      </div>
    </div>
  </div>
</body>
</html>`, escaped)
}

func detectLikelyHTML(message string) bool {
	s := strings.TrimSpace(strings.ToLower(message))
	if s == "" {
		return false
	}
	if strings.Contains(s, "<!doctype") || strings.Contains(s, "<html") {
		return true
	}
	// Common tags
	return strings.Contains(s, "<p") ||
		strings.Contains(s, "<div") ||
		strings.Contains(s, "<br") ||
		strings.Contains(s, "<h1") ||
		strings.Contains(s, "<h2") ||
		strings.Contains(s, "<h3") ||
		strings.Contains(s, "<ul") ||
		strings.Contains(s, "<ol") ||
		strings.Contains(s, "<li") ||
		strings.Contains(s, "<strong") ||
		strings.Contains(s, "<a ")
}

func newJobID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// fallback (still unique enough for in-memory usage)
		return hex.EncodeToString([]byte(time.Now().UTC().Format(time.RFC3339Nano)))
	}
	return hex.EncodeToString(b)
}

