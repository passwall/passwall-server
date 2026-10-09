package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	"github.com/passwall/passwall-server/internal/service"
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
