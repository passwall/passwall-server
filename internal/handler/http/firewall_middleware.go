package http

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/passwall/passwall-server/internal/service"
	"github.com/passwall/passwall-server/pkg/constants"
)

// FirewallMiddleware checks organization-level firewall rules for org-scoped routes.
// It reads the resolved numeric org ID from gin context (set by OrgPublicIDResolverMiddleware)
// and checks the client IP against the org's firewall policy.
func FirewallMiddleware(firewallService service.PolicyFirewallService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if firewallService == nil {
			c.Next()
			return
		}

		val, exists := c.Get(constants.ContextKeyOrgID)
		if !exists {
			c.Next()
			return
		}

		orgID, ok := val.(uint)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid organization context"})
			c.Abort()
			return
		}

		if !enforceFirewall(c, firewallService, orgID) {
			return
		}
		c.Next()
	}
}

// enforceFirewall checks the client IP against an organization's firewall
// policy and aborts the request when it is not allowed or cannot be checked.
func enforceFirewall(c *gin.Context, firewallService service.PolicyFirewallService, orgID uint) bool {
	result, err := firewallService.CheckAccess(c.Request.Context(), orgID, GetIPAddress(c))
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "firewall check failed"})
		c.Abort()
		return false
	}
	if !result.Allowed {
		c.JSON(http.StatusForbidden, gin.H{
			"error":  "access denied by organization firewall policy",
			"code":   "FIREWALL_DENIED",
			"reason": result.Reason,
		})
		c.Abort()
		return false
	}
	return true
}

// FirewallForResourceMiddleware applies the owning organization's firewall to
// routes that address a resource by ID (/org-items/:id, /collections/:id,
// /teams/:id). Unknown IDs pass through so the handler can answer 404.
func FirewallForResourceMiddleware(firewallService service.PolicyFirewallService, kind service.FirewallResource) gin.HandlerFunc {
	return func(c *gin.Context) {
		if firewallService == nil {
			c.Next()
			return
		}
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil || id == 0 {
			c.Next()
			return
		}
		orgID, err := firewallService.ResolveOrganization(c.Request.Context(), kind, uint(id))
		if err != nil || orgID == 0 {
			c.Next()
			return
		}
		if !enforceFirewall(c, firewallService, orgID) {
			return
		}
		c.Next()
	}
}
