package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	"github.com/passwall/passwall-server/internal/service"
	"github.com/passwall/passwall-server/pkg/constants"
)

func TestAdminMailRejectsFreeFormRecipients(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewAdminMailHandler(nil, nil, nil)

	for name, body := range map[string]string{
		"legacy external recipients": `{"recipients":["victim@example.com"],"subject":"hi","message":"hello"}`,
		"missing send_to":            `{"subject":"hi","message":"hello"}`,
	} {
		t.Run(name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/api/admin/mail", strings.NewReader(body))
			handler.CreateJob(c)
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body=%s", recorder.Code, recorder.Body.String())
			}
			if len(handler.jobs) != 0 {
				t.Fatal("a mail job was created")
			}
		})
	}
}

type activityLimitServiceStub struct {
	service.UserActivityService
	userLimit   int
	filterLimit int
}

func (s *activityLimitServiceStub) GetUserActivities(_ context.Context, _ uint, limit int) ([]*domain.UserActivity, error) {
	s.userLimit = limit
	return nil, nil
}

func (s *activityLimitServiceStub) ListActivities(_ context.Context, filter repository.ActivityFilter) ([]*domain.UserActivity, int64, error) {
	s.filterLimit = filter.Limit
	return nil, 0, nil
}

func TestAdminActivityLimitsAreBounded(t *testing.T) {
	gin.SetMode(gin.TestMode)
	stub := &activityLimitServiceStub{}
	handler := NewActivityHandler(stub)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/activities?limit=100000", nil)
	handler.ListActivities(c)

	recorder = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(recorder)
	c.Params = gin.Params{{Key: "id", Value: "7"}}
	c.Request = httptest.NewRequest(http.MethodGet, "/api/users/7/activities?limit=100000", nil)
	handler.GetUserActivities(c)

	if stub.filterLimit != maxAdminActivityLimit || stub.userLimit != maxAdminActivityLimit {
		t.Fatalf("limits = list %d, user %d; want %d", stub.filterLimit, stub.userLimit, maxAdminActivityLimit)
	}
}

type stepUpAuthStub struct {
	service.AuthService
	valid string
}

func (s stepUpAuthStub) VerifyAdminStepUp(token string, _ uint, _ uuid.UUID) error {
	if token != "" && token == s.valid {
		return nil
	}
	return service.ErrStepUpInvalid
}

type auditActivityStub struct {
	service.UserActivityService
	entries []*domain.CreateActivityRequest
}

func (s *auditActivityStub) LogActivity(_ context.Context, req *domain.CreateActivityRequest) error {
	s.entries = append(s.entries, req)
	return nil
}

func TestAdminStepUpAndAuditMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	activities := &auditActivityStub{}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(constants.ContextKeyUserID, uint(42))
		c.Set(constants.ContextKeySessionID, uuid.New())
	})
	router.POST("/danger/:id",
		RequireAdminStepUpMiddleware(stepUpAuthStub{valid: "good"}),
		AdminAuditMiddleware(service.NewActivityLogger(activities), domain.ActivityTypeAdminMailSent),
		func(c *gin.Context) {
			SetAdminAuditDetails(c, service.ActivityDetails{"recipient_count": 3})
			c.Status(http.StatusAccepted)
		})

	for _, tt := range []struct {
		header string
		want   int
	}{{"", http.StatusForbidden}, {"bad", http.StatusForbidden}, {"good", http.StatusAccepted}} {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/danger/7", nil)
		if tt.header != "" {
			req.Header.Set(AdminStepUpHeader, tt.header)
		}
		router.ServeHTTP(recorder, req)
		if recorder.Code != tt.want {
			t.Fatalf("header %q: status = %d, want %d", tt.header, recorder.Code, tt.want)
		}
		if tt.want == http.StatusForbidden && !strings.Contains(recorder.Body.String(), `"STEP_UP_REQUIRED"`) {
			t.Fatalf("missing STEP_UP_REQUIRED code: %s", recorder.Body.String())
		}
	}

	if len(activities.entries) != 1 {
		t.Fatalf("audit entries = %d, want 1 (only the request that reached the handler)", len(activities.entries))
	}
	entry := activities.entries[0]
	if entry.UserID != 42 || entry.ActivityType != domain.ActivityTypeAdminMailSent ||
		!strings.Contains(entry.Details, `"recipient_count":3`) || !strings.Contains(entry.Details, `"param_id":"7"`) ||
		!strings.Contains(entry.Details, `"status":202`) {
		t.Fatalf("audit entry = %+v", entry)
	}
}
