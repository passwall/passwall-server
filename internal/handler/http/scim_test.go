package http

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/service"
	"github.com/stretchr/testify/require"
)

type scimAuthServiceStub struct {
	service.SCIMService
	err error
}

func (s scimAuthServiceStub) ValidateToken(context.Context, string) (uint, error) {
	return 0, s.err
}

func TestRespondSCIMEntitlementError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	err := &service.EntitlementDeniedError{
		Capability: domain.CapabilitySSOManage,
		Reason:     domain.EntitlementReasonAccountFrozen,
		Cause:      service.ErrAccountFrozen,
	}

	require.True(t, respondSCIMEntitlementError(context, err))
	require.Equal(t, http.StatusForbidden, recorder.Code)
	require.JSONEq(t, `{
		"schemas": ["urn:ietf:params:scim:api:messages:2.0:Error"],
		"detail": "Organization account is frozen",
		"status": "403"
	}`, recorder.Body.String())
}

func TestRespondSCIMEntitlementErrorIgnoresUnrelatedErrors(t *testing.T) {
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)

	require.False(t, respondSCIMEntitlementError(context, errors.New("database unavailable")))
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Empty(t, recorder.Body.String())
}

func TestSCIMAuthMiddlewareReturnsSCIMForbiddenForEntitlementDenial(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(SCIMAuthMiddleware(scimAuthServiceStub{
		err: &service.EntitlementDeniedError{
			Capability: domain.CapabilitySCIMManage,
			Reason:     domain.EntitlementReasonAccountFrozen,
			Cause:      service.ErrAccountFrozen,
		},
	}))
	router.GET("/scim/v2/Users", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	request := httptest.NewRequest(http.MethodGet, "/scim/v2/Users", nil)
	request.Header.Set("Authorization", "Bearer pwscim_test")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusForbidden, recorder.Code)
	require.JSONEq(t, `{
		"schemas": ["urn:ietf:params:scim:api:messages:2.0:Error"],
		"detail": "Organization account is frozen",
		"status": "403"
	}`, recorder.Body.String())
}
