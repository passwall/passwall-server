package http

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/passwall/passwall-server/internal/service"
	"github.com/passwall/passwall-server/pkg/constants"
	"github.com/passwall/passwall-server/pkg/database"
)

// AuthMiddleware validates JWT tokens and extracts user information
func AuthMiddleware(authService service.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "authorization header required"})
			c.Abort()
			return
		}

		// Extract Bearer token
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization header format"})
			c.Abort()
			return
		}

		token := parts[1]

		// Validate token
		claims, err := authService.ValidateToken(c.Request.Context(), token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			c.Abort()
			return
		}

		// Set user information in context (using constants for keys)
		c.Set(constants.ContextKeyUserID, claims.UserID)
		c.Set(constants.ContextKeyEmail, claims.Email)
		c.Set(constants.ContextKeySchema, claims.Schema)
		c.Set(constants.ContextKeyUserRole, claims.Role)
		c.Set(constants.ContextKeyTokenUUID, claims.UUID.String())

		// Determine which schema to use
		schemaToUse := claims.Schema

		// Admin users can override schema using X-User-Schema header
		if constants.IsAdmin(claims.Role) {
			customSchema := c.GetHeader("X-User-Schema")
			if customSchema != "" {
				// Validate that the custom schema exists
				if err := authService.ValidateSchema(c.Request.Context(), customSchema); err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": "invalid schema"})
					c.Abort()
					return
				}
				schemaToUse = customSchema
			}
		}

		// Set schema in request context for repository/service access
		ctx := database.WithSchema(c.Request.Context(), schemaToUse)
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}

// CORSMiddleware handles CORS
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With, X-User-Schema")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE, PATCH")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

// SecurityMiddleware adds security headers
// NOTE: CSP is NOT included here because backend is an API server (not HTML server)
// CSP should be set in frontend HTML meta tags or via frontend web server (nginx/cloudflare)
func SecurityMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Basic security headers (safe for API servers)
		c.Writer.Header().Set("X-Content-Type-Options", "nosniff")
		c.Writer.Header().Set("X-Frame-Options", "DENY")
		c.Writer.Header().Set("X-XSS-Protection", "1; mode=block")

		// HSTS (only for HTTPS - safe to set even on HTTP, browser ignores it)
		c.Writer.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")

		c.Next()
	}
}
