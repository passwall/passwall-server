package http

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/passwall/passwall-server/pkg/constants"
)

// GetUintParam extracts a uint parameter from the URL
// Returns the parsed uint and true if successful
// Automatically responds with 400 Bad Request if invalid
func GetUintParam(c *gin.Context, paramName string) (uint, bool) {
	paramStr := c.Param(paramName)
	if paramStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": paramName + " is required"})
		return 0, false
	}

	id, err := strconv.ParseUint(paramStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid " + paramName})
		return 0, false
	}

	return uint(id), true
}

// GetIntParam extracts an int parameter from the URL
func GetIntParam(c *gin.Context, paramName string) (int, bool) {
	paramStr := c.Param(paramName)
	if paramStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": paramName + " is required"})
		return 0, false
	}

	val, err := strconv.Atoi(paramStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid " + paramName})
		return 0, false
	}

	return val, true
}

// GetStringParam extracts a string parameter from the URL
// Returns empty string and false if not found
func GetStringParam(c *gin.Context, paramName string) (string, bool) {
	val := c.Param(paramName)
	if val == "" {
		return "", false
	}
	return val, true
}

// GetResolvedOrgID retrieves the numeric organization ID that was resolved
// from a public_id by OrgPublicIDResolverMiddleware. Returns 0, false if the
// middleware has not run or the value is missing.
func GetResolvedOrgID(c *gin.Context) (uint, bool) {
	val, exists := c.Get(constants.ContextKeyOrgID)
	if !exists {
		c.JSON(http.StatusBadRequest, gin.H{"error": "organization identifier is required"})
		return 0, false
	}
	orgID, ok := val.(uint)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid organization context"})
		return 0, false
	}
	return orgID, true
}

// GetCurrentUserID extracts user ID from context (helper for handlers)
// Panics if user ID not found (should never happen after auth middleware)
func GetCurrentUserID(c *gin.Context) uint {
	userID, err := GetUserID(c)
	if err != nil {
		// This should never happen if auth middleware is working correctly
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		panic(err)
	}
	return userID
}
