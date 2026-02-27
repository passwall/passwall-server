package http

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/passwall/passwall-server/internal/service"
)

// FirewallMiddleware checks organization-level firewall rules for org-scoped routes.
// It reads :id as the organization ID from the URL and checks the client IP.
func FirewallMiddleware(firewallService service.PolicyFirewallService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if firewallService == nil {
			c.Next()
			return
		}

		orgIDStr := c.Param("id")
		if orgIDStr == "" {
			c.Next()
			return
		}

		orgID, err := strconv.ParseUint(orgIDStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid organization ID"})
			c.Abort()
			return
		}

		clientIP := GetIPAddress(c)

		result, err := firewallService.CheckAccess(c.Request.Context(), uint(orgID), clientIP)
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
