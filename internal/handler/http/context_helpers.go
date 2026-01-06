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

// GetIPAddress extracts client IP from request
func GetIPAddress(c *gin.Context) string {
	// Check X-Forwarded-For header (if behind proxy)
	if forwarded := c.GetHeader("X-Forwarded-For"); forwarded != "" {
		return forwarded
	}

	// Check X-Real-IP header
	if realIP := c.GetHeader("X-Real-IP"); realIP != "" {
		return realIP
	}

	// Fallback to RemoteAddr
	return c.ClientIP()
}

// GetUserAgent extracts user agent from request
func GetUserAgent(c *gin.Context) string {
	return c.GetHeader("User-Agent")
}
