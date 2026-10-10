package http

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/passwall/passwall-server/pkg/constants"
)

// GetUserID extracts user ID from Gin context (set by AuthMiddleware)
func GetUserID(c *gin.Context) (uint, error) {
	userID, exists := c.Get(constants.ContextKeyUserID)
	if !exists {
		return 0, fmt.Errorf("user ID not found in context")
	}

	id, ok := userID.(uint)
	if !ok {
		return 0, fmt.Errorf("invalid user ID type")
	}

	return id, nil
}

// GetIPAddress returns the client IP. Forwarding headers are only honored
// when the request comes from a trusted proxy (see core.ConfigureClientIP);
// otherwise a client could pick its own IP and bypass IP-based policies
// (firewall rules, failed-login limits) or forge audit entries.
func GetIPAddress(c *gin.Context) string {
	return c.ClientIP()
}

// GetUserAgent extracts user agent from request
func GetUserAgent(c *gin.Context) string {
	return c.GetHeader("User-Agent")
}
