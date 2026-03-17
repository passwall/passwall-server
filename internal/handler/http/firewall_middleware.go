package http

import (
	"net/http"

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

		clientIP := GetIPAddress(c)

		result, err := firewallService.CheckAccess(c.Request.Context(), orgID, clientIP)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "firewall check failed"})
			c.Abort()
			return
		}

		if !result.Allowed {
			c.JSON(http.StatusForbidden, gin.H{
				"error":  "access denied by organization firewall policy",
				"reason": result.Reason,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
