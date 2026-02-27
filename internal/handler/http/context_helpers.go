package http

import (
	"fmt"
	"strings"

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

// GetIPAddress extracts the original client IP from request headers.
// X-Forwarded-For may contain "client, proxy1, proxy2"; we take the first entry.
func GetIPAddress(c *gin.Context) string {
	if forwarded := c.GetHeader("X-Forwarded-For"); forwarded != "" {
		if first, _, ok := strings.Cut(forwarded, ","); ok {
			return strings.TrimSpace(first)
		}
		return strings.TrimSpace(forwarded)
	}

	if realIP := c.GetHeader("X-Real-IP"); realIP != "" {
		return strings.TrimSpace(realIP)
	}

	return c.ClientIP()
}

// GetUserAgent extracts user agent from request
func GetUserAgent(c *gin.Context) string {
	return c.GetHeader("User-Agent")
}
