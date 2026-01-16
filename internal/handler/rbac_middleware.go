package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/passwall/passwall-server/internal/service"
)

// RequireOrgPermission is a middleware that checks if the user has a specific permission in an organization
func RequireOrgPermission(permService service.PermissionService, permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get user ID from context (set by auth middleware)
		userID, exists := c.Get("userID")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			c.Abort()
			return
		}

		// Get organization ID from URL parameter
		orgIDStr := c.Param("org_id")
		if orgIDStr == "" {
			orgIDStr = c.Param("id") // Fallback for /organizations/:id routes
		}

		if orgIDStr == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Organization ID required"})
			c.Abort()
			return
		}

		orgID, err := strconv.ParseUint(orgIDStr, 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid organization ID"})
			c.Abort()
			return
		}

		// Check permission
		can, err := permService.Can(c.Request.Context(), userID.(uint), uint(orgID), permission)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			c.Abort()
			return
		}

		if !can {
			c.JSON(http.StatusForbidden, gin.H{
				"error":               "Insufficient permissions",
				"required_permission": permission,
			})
			c.Abort()
			return
		}

		// Store organization ID in context for handlers to use
		c.Set("orgID", uint(orgID))

		c.Next()
	}
}

// RequireOrgMembership checks if the user is a member of the organization
func RequireOrgMembership(permService service.PermissionService) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get user ID from context
		userID, exists := c.Get("userID")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			c.Abort()
			return
		}

		// Get organization ID from URL parameter
		orgIDStr := c.Param("org_id")
		if orgIDStr == "" {
			orgIDStr = c.Param("id")
		}

		if orgIDStr == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Organization ID required"})
			c.Abort()
			return
		}

		orgID, err := strconv.ParseUint(orgIDStr, 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid organization ID"})
			c.Abort()
			return
		}

		// Check if user has any role (even read-only)
		role, err := permService.GetEffectiveRole(c.Request.Context(), userID.(uint), uint(orgID))
		if err != nil {
			c.JSON(http.StatusForbidden, gin.H{"error": "Not a member of this organization"})
			c.Abort()
			return
		}

		// Store role and organization ID in context
		c.Set("orgID", uint(orgID))
		c.Set("orgRole", role)

		c.Next()
	}
}

// GetUserIDFromContext retrieves the user ID from Gin context
func GetUserIDFromContext(c *gin.Context) (uint, bool) {
	userID, exists := c.Get("userID")
	if !exists {
		return 0, false
	}
	return userID.(uint), true
}

// GetOrgIDFromContext retrieves the organization ID from Gin context
func GetOrgIDFromContext(c *gin.Context) (uint, bool) {
	orgID, exists := c.Get("orgID")
	if !exists {
		return 0, false
	}
	return orgID.(uint), true
}
