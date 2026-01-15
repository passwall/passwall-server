package http

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"net/mail"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/passwall/passwall-server/internal/email"
	"github.com/passwall/passwall-server/internal/repository"
)

type AdminMailHandler struct {
	emailSender email.Sender
	logger      email.Logger
	userRepo    repository.UserRepository

	mu   sync.RWMutex
	jobs map[string]*mailJob
}

type mailJobState string

const (
	mailJobQueued   mailJobState = "queued"
	mailJobRunning  mailJobState = "running"
	mailJobFinished mailJobState = "finished"
	mailJobFailed   mailJobState = "failed"
)

type mailFailure struct {
	Email string `json:"email"`
	Error string `json:"error"`
}

type mailJob struct {
	ID        string       `json:"job_id"`
	State     mailJobState `json:"state"`
	CreatedAt time.Time    `json:"created_at"`
	StartedAt *time.Time   `json:"started_at,omitempty"`
	EndedAt   *time.Time   `json:"ended_at,omitempty"`

	Total  int `json:"total"`
	Sent   int `json:"sent"`
	Failed int `json:"failed"`

	Failures []mailFailure `json:"failures"`

	Subject string `json:"subject"`

	// Human-friendly failure summary (e.g. exceeded recipient limit)
	Error *string `json:"error,omitempty"`
}

// Legacy request (kept for backward compatibility with /admin/bulk-email)
type AdminBulkEmailCreateRequest struct {
	Recipients []string `json:"recipients" binding:"required,min=1,max=1500"`
	Subject    string   `json:"subject" binding:"required,min=1,max=200"`
	Message    string   `json:"message" binding:"required,min=1,max=20000"`
	IsHTML     *bool    `json:"is_html"`
}

type AdminMailCreateRequest struct {
	// send_to determines recipient source:
	// - "all_users": sends to users in the system (optionally filtered by search)
	// - "user_ids": sends to specific users by their numeric IDs
	SendTo string `json:"send_to"`

	UserIDs []uint `json:"user_ids"`
	Search  string `json:"search"`

	Subject string `json:"subject"`
	Message string `json:"message"`
	IsHTML  *bool  `json:"is_html"`
}

type AdminMailCreateResponse struct {
	JobID string `json:"job_id"`
	Total int    `json:"total"`
}

func NewAdminMailHandler(emailSender email.Sender, userRepo repository.UserRepository, logger email.Logger) *AdminMailHandler {
	return &AdminMailHandler{
		emailSender: emailSender,
		userRepo:    userRepo,
		logger:      logger,
		jobs:        make(map[string]*mailJob),
	}
}

func (h *AdminMailHandler) CreateJob(c *gin.Context) {
	bodyBytes, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	// Try new request shape first, then fall back to legacy bulk-email request.
	var req AdminMailCreateRequest
	reqErr := json.Unmarshal(bodyBytes, &req)

	if reqErr != nil || (strings.TrimSpace(req.SendTo) == "" && len(req.UserIDs) == 0 && strings.TrimSpace(req.Subject) == "" && strings.TrimSpace(req.Message) == "") {
		var legacy AdminBulkEmailCreateRequest
		if err := json.Unmarshal(bodyBytes, &legacy); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "invalid request",
				"details": err.Error(),
			})
			return
		}
		h.createLegacyJob(c, legacy)
		return
	}

	h.createMailJob(c, req)
}

func (h *AdminMailHandler) createLegacyJob(c *gin.Context, req AdminBulkEmailCreateRequest) {
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

	job := &mailJob{
		ID:        jobID,
		State:     mailJobQueued,
		CreatedAt: now,
		Total:     len(recipients),
		Failures:  make([]mailFailure, 0),
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
	go h.runExplicitRecipientsJob(context.Background(), jobID, recipients, subject, message, isHTML)

	h.logger.Info("admin mail job created",
		"job_id", jobID,
		"recipient_count", len(recipients),
		"source", "explicit_recipients",
	)

	c.JSON(http.StatusOK, AdminMailCreateResponse{
		JobID: jobID,
		Total: len(recipients),
	})
}

func (h *AdminMailHandler) createMailJob(c *gin.Context, req AdminMailCreateRequest) {
	sendTo := strings.TrimSpace(strings.ToLower(req.SendTo))
	if sendTo == "" {
		sendTo = "all_users"
	}
	if sendTo != "all_users" && sendTo != "user_ids" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "send_to must be one of: all_users, user_ids"})
		return
	}

	subject := strings.TrimSpace(req.Subject)
	if subject == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "subject is required"})
		return
	}
	if len(subject) > 200 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "subject is too long (max 200)"})
		return
	}

	message := strings.TrimSpace(req.Message)
	if message == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "message is required"})
		return
	}
	if len(message) > 20000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "message is too long (max 20000)"})
		return
	}

	isHTML := false
	if req.IsHTML != nil {
		isHTML = *req.IsHTML
	} else {
		isHTML = detectLikelyHTML(message)
	}

	switch sendTo {
	case "user_ids":
		if len(req.UserIDs) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "user_ids is required when send_to=user_ids"})
			return
		}
		if len(req.UserIDs) > 1500 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "too many user_ids (max 1500)"})
			return
		}

		recipients, missingIDs, err := h.resolveEmailsByUserIDs(c.Request.Context(), req.UserIDs)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve recipients"})
			return
		}
		if len(missingIDs) > 0 {
			sample := missingIDs
			if len(sample) > 50 {
				sample = sample[:50]
			}
			c.JSON(http.StatusBadRequest, gin.H{
				"error":         "some users not found",
				"missing_count": len(missingIDs),
				"missing_ids":   sample,
			})
			return
		}
		if len(recipients) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "no recipients found for given user_ids"})
			return
		}

		jobID := newJobID()
		now := time.Now().UTC()
		job := &mailJob{
			ID:        jobID,
			State:     mailJobQueued,
			CreatedAt: now,
			Total:     len(recipients),
			Failures:  make([]mailFailure, 0),
			Subject:   subject,
		}
		h.mu.Lock()
		h.jobs[jobID] = job
		h.mu.Unlock()

		go h.runExplicitRecipientsJob(context.Background(), jobID, recipients, subject, message, isHTML)

		h.logger.Info("admin mail job created",
			"job_id", jobID,
			"recipient_count", len(recipients),
			"source", "user_ids",
		)

		c.JSON(http.StatusOK, AdminMailCreateResponse{JobID: jobID, Total: len(recipients)})
		return

	case "all_users":
		jobID := newJobID()
		now := time.Now().UTC()
		job := &mailJob{
			ID:        jobID,
			State:     mailJobQueued,
			CreatedAt: now,
			Total:     0, // updated as we discover recipients
			Failures:  make([]mailFailure, 0),
			Subject:   subject,
		}
		h.mu.Lock()
		h.jobs[jobID] = job
		h.mu.Unlock()

		search := strings.TrimSpace(req.Search)
		go h.runAllUsersJob(context.Background(), jobID, subject, message, isHTML, search)

		h.logger.Info("admin mail job created",
			"job_id", jobID,
			"source", "all_users",
			"search", search,
		)

		c.JSON(http.StatusOK, AdminMailCreateResponse{JobID: jobID, Total: 0})
		return
	}
}

func (h *AdminMailHandler) resolveEmailsByUserIDs(ctx context.Context, ids []uint) ([]string, []uint, error) {
	seen := make(map[string]struct{}, len(ids))
	out := make([]string, 0, len(ids))
	missing := make([]uint, 0)

	for _, id := range ids {
		user, err := h.userRepo.GetByID(ctx, id)
		if err != nil {
			if err == repository.ErrNotFound {
				missing = append(missing, id)
				continue
			}
			return nil, nil, err
		}
		emailStr := strings.ToLower(strings.TrimSpace(user.Email))
		if emailStr == "" {
			continue
		}
		if _, err := mail.ParseAddress(emailStr); err != nil {
			continue
		}
		if _, ok := seen[emailStr]; ok {
			continue
		}
		seen[emailStr] = struct{}{}
		out = append(out, emailStr)
	}

	return out, missing, nil
}

func (h *AdminMailHandler) GetJob(c *gin.Context) {
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

func (h *AdminMailHandler) runExplicitRecipientsJob(
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
	job.State = mailJobRunning
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
				job.Failures = append(job.Failures, mailFailure{
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
		job.State = mailJobFailed
	} else {
		job.State = mailJobFinished
	}

	h.logger.Info("admin mail job finished",
		"job_id", jobID,
		"total", job.Total,
		"sent", job.Sent,
		"failed", job.Failed,
	)
}

func (h *AdminMailHandler) runAllUsersJob(
	ctx context.Context,
	jobID string,
	subject string,
	message string,
	isHTML bool,
	search string,
) {
	startedAt := time.Now().UTC()
	h.mu.Lock()
	job, ok := h.jobs[jobID]
	if !ok {
		h.mu.Unlock()
		return
	}
	job.State = mailJobRunning
	job.StartedAt = &startedAt
	h.mu.Unlock()

	const pageSize = 200
	const batchSize = 10
	const batchDelay = 1100 * time.Millisecond

	body := buildAdminBroadcastBody(message, isHTML)

	seen := make(map[string]struct{}, 1024)
	sendQueue := make([]string, 0, batchSize)

	flush := func() bool {
		if len(sendQueue) == 0 {
			return true
		}
		for _, to := range sendQueue {
			if ctx.Err() != nil {
				return false
			}
			err := h.sendWithRetry(ctx, to, subject, body)
			h.mu.Lock()
			if err != nil {
				job.Failed++
				job.Failures = append(job.Failures, mailFailure{Email: to, Error: err.Error()})
			} else {
				job.Sent++
			}
			h.mu.Unlock()
		}
		sendQueue = sendQueue[:0]
		time.Sleep(batchDelay)
		return true
	}

	for offset := 0; ; offset += pageSize {
		if ctx.Err() != nil {
			break
		}

		users, _, err := h.userRepo.List(ctx, repository.ListFilter{
			Search: search,
			Limit:  pageSize,
			Offset: offset,
			Sort:   "id",
			Order:  "asc",
		})
		if err != nil {
			msg := "failed to list users"
			h.mu.Lock()
			job.Error = &msg
			job.State = mailJobFailed
			h.mu.Unlock()
			break
		}
		if len(users) == 0 {
			break
		}

		for _, u := range users {
			emailStr := strings.ToLower(strings.TrimSpace(u.Email))
			if emailStr == "" {
				continue
			}
			if _, err := mail.ParseAddress(emailStr); err != nil {
				continue
			}
			if _, ok := seen[emailStr]; ok {
				continue
			}

			seen[emailStr] = struct{}{}
			h.mu.Lock()
			job.Total++
			h.mu.Unlock()

			sendQueue = append(sendQueue, emailStr)
			if len(sendQueue) >= batchSize {
				if ok := flush(); !ok {
					goto done
				}
			}
		}
	}

	_ = flush()

done:
	endedAt := time.Now().UTC()
	h.mu.Lock()
	defer h.mu.Unlock()
	job, ok = h.jobs[jobID]
	if !ok {
		return
	}
	job.EndedAt = &endedAt
	if job.State != mailJobFailed {
		if job.Failed > 0 && job.Sent == 0 {
			job.State = mailJobFailed
		} else {
			job.State = mailJobFinished
		}
	}

	h.logger.Info("admin mail job finished",
		"job_id", jobID,
		"total", job.Total,
		"sent", job.Sent,
		"failed", job.Failed,
	)
}

func (h *AdminMailHandler) sendWithRetry(ctx context.Context, to, subject, body string) error {
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
