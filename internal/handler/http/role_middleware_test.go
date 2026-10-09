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
	"github.com/passwall/passwall-server/pkg/constants"
)

type systemAdminUserRepo struct {
	repository.UserRepository
	user *domain.User
}

func TestWriteManualSubscriptionErrorMapsExternalConflict(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	writeManualSubscriptionError(context, &service.ManualSubscriptionError{
		Code:    service.ManualSubscriptionCodeExternalActive,
		Message: "blocked",
	})
	if recorder.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", recorder.Code)
	}
	if body := recorder.Body.String(); body == "" ||
		!containsAll(body, `"code":"EXTERNAL_SUBSCRIPTION_ACTIVE"`, `"error":"blocked"`) {
		t.Fatalf("unexpected body: %s", body)
	}
}

func containsAll(value string, expected ...string) bool {
	for _, item := range expected {
		if !strings.Contains(value, item) {
			return false
		}
	}
	return true
}

func (r systemAdminUserRepo) GetByID(context.Context, uint) (*domain.User, error) {
	return r.user, nil
}

func TestRequireSystemAdminMiddlewareRequiresDatabaseFlag(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tt := range []struct {
		name       string
		tokenRole  string
		dbRoleID   uint
		systemUser bool
		wantStatus int
	}{
		{name: "ordinary admin denied", tokenRole: constants.RoleAdmin, dbRoleID: constants.RoleIDAdmin, wantStatus: http.StatusForbidden},
		{name: "system user without admin role denied", tokenRole: constants.RoleMember, dbRoleID: constants.RoleIDMember, systemUser: true, wantStatus: http.StatusForbidden},
		{name: "stale admin token after demotion denied", tokenRole: constants.RoleAdmin, dbRoleID: constants.RoleIDMember, systemUser: true, wantStatus: http.StatusForbidden},
		{name: "system admin allowed", tokenRole: constants.RoleAdmin, dbRoleID: constants.RoleIDAdmin, systemUser: true, wantStatus: http.StatusNoContent},
	} {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set(constants.ContextKeyUserID, uint(42))
				c.Set(constants.ContextKeyUserRole, tt.tokenRole)
			})
			router.POST("/mutate", RequireSystemAdminMiddleware(systemAdminUserRepo{
				user: &domain.User{ID: 42, RoleID: tt.dbRoleID, IsSystemUser: tt.systemUser},
			}), func(c *gin.Context) {
				c.Status(http.StatusNoContent)
			})

			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/mutate", nil))
			if recorder.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", recorder.Code, tt.wantStatus)
			}
		})
	}
}
