package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/passwall/passwall-server/internal/service"
	"github.com/stretchr/testify/assert"
)

type fakeFirewall struct {
	blockedOrg uint
	owners     map[uint]uint
	failCheck  bool
}

func (f fakeFirewall) CheckAccess(_ context.Context, orgID uint, _ string) (*service.FirewallCheckResult, error) {
	if f.failCheck {
		return nil, assert.AnError
	}
	return &service.FirewallCheckResult{Allowed: orgID != f.blockedOrg}, nil
}

func (f fakeFirewall) ResolveOrganization(_ context.Context, _ service.FirewallResource, id uint) (uint, error) {
	return f.owners[id], nil
}

func TestFirewallForResourceMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	run := func(fw fakeFirewall, path string) int {
		router := gin.New()
		router.GET("/org-items/:id", FirewallForResourceMiddleware(fw, service.FirewallResourceItem), func(c *gin.Context) { c.Status(http.StatusOK) })
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		return rec.Code
	}
	fw := fakeFirewall{blockedOrg: 7, owners: map[uint]uint{1: 7, 2: 8}}
	assert.Equal(t, http.StatusForbidden, run(fw, "/org-items/1"), "item of a firewalled org")
	assert.Equal(t, http.StatusOK, run(fw, "/org-items/2"))
	assert.Equal(t, http.StatusOK, run(fw, "/org-items/999"), "unknown items reach the handler (404)")
	fw.failCheck = true
	assert.Equal(t, http.StatusServiceUnavailable, run(fw, "/org-items/2"), "fails closed")
}
