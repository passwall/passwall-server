package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/passwall/passwall-server/internal/repository"
	"github.com/passwall/passwall-server/pkg/constants"
)

// RequireAdminMiddleware ensures only admin users can access the route
func RequireAdminMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get user role from context (set by AuthMiddleware)
		role, exists := c.Get(constants.ContextKeyUserRole)
		if !exists {
			c.JSON(http.StatusForbidden, gin.H{"error": "role not found in context"})
			c.Abort()
			return
		}

		// Check if user is admin (using constant)
		if !constants.IsAdmin(role.(string)) {
			c.JSON(http.StatusForbidden, gin.H{"error": "admin access required"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequireSystemAdminMiddleware verifies the current user against the database.
// The system-user flag deliberately is not carried in or trusted from JWT claims.
func RequireSystemAdminMiddleware(userRepo repository.UserRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, roleExists := c.Get(constants.ContextKeyUserRole)
		userID, userExists := c.Get(constants.ContextKeyUserID)
		roleName, roleOK := role.(string)
		id, idOK := userID.(uint)
		if !roleExists || !userExists || !roleOK || !idOK || !constants.IsAdmin(roleName) {
			c.JSON(http.StatusForbidden, gin.H{"error": "system admin access required"})
			c.Abort()
			return
		}

		user, err := userRepo.GetByID(c.Request.Context(), id)
		if err != nil {
			c.JSON(http.StatusForbidden, gin.H{"error": "system admin access required"})
			c.Abort()
			return
		}
		if !user.IsAdmin() || !user.IsSystemUser {
			c.JSON(http.StatusForbidden, gin.H{"error": "system admin access required"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequireRolesMiddleware ensures user has one of the specified roles
func RequireRolesMiddleware(allowedRoles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get user role from context (using constant)
		role, exists := c.Get(constants.ContextKeyUserRole)
		if !exists {
			c.JSON(http.StatusForbidden, gin.H{"error": "role not found in context"})
			c.Abort()
			return
		}

		// Check if user has one of the allowed roles
		userRole := role.(string)
		for _, allowedRole := range allowedRoles {
			if userRole == allowedRole {
				c.Next()
				return
			}
		}

		c.JSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
		c.Abort()
	}
}
