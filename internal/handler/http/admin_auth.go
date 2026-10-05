package http

import (
	"crypto/hmac"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// AdminServiceAuthMiddleware authenticates the read-only admin directory API.
//
// Callers must send Authorization: Bearer <server.admin_api_key>. The comparison
// follows the same constant-time bearer-secret check used for the RevenueCat webhook.
// End-user JWTs, including tokens for users with the admin role, are not accepted.
// An empty configured key disables the API instead of falling open.
func AdminServiceAuthMiddleware(apiKey string) gin.HandlerFunc {
	expected := strings.TrimSpace(apiKey)
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")

		if expected == "" {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "admin directory api is disabled"})
			c.Abort()
			return
		}

		token, ok := directoryBearerToken(c.GetHeader("Authorization"))
		if !ok || !hmac.Equal([]byte(token), []byte(expected)) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or missing admin credential"})
			c.Abort()
			return
		}

		c.Next()
	}
}

func directoryBearerToken(header string) (string, bool) {
	parts := strings.SplitN(strings.TrimSpace(header), " ", 2)
	if len(parts) != 2 || parts[0] != "Bearer" {
		return "", false
	}
	token := strings.TrimSpace(parts[1])
	if token == "" {
		return "", false
	}
	return token, true
}
