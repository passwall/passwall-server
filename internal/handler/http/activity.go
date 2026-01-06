package http

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	"github.com/passwall/passwall-server/internal/service"
	"github.com/passwall/passwall-server/pkg/constants"
)

type ActivityHandler struct {
	activityService service.UserActivityService
}

// NewActivityHandler creates a new activity handler
func NewActivityHandler(activityService service.UserActivityService) *ActivityHandler {
	return &ActivityHandler{
		activityService: activityService,
	}
}

// GetMyActivities returns current user's activities
func (h *ActivityHandler) GetMyActivities(c *gin.Context) {
	// Get user ID from context
	userID, exists := c.Get(constants.ContextKeyUserID)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	// Parse limit
	limit := 50 // Default
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	activities, err := h.activityService.GetUserActivities(c.Request.Context(), userID.(uint), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get activities"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"activities": domain.ToUserActivityDTOs(activities),
		"count":      len(activities),
	})
}

// GetLastSignIn returns user's last signin activity
func (h *ActivityHandler) GetLastSignIn(c *gin.Context) {
	userID, exists := c.Get(constants.ContextKeyUserID)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	activity, err := h.activityService.GetLastSignIn(c.Request.Context(), userID.(uint))
	if err != nil {
		if err == repository.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "no signin activity found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get last signin"})
		return
	}

	c.JSON(http.StatusOK, domain.ToUserActivityDTO(activity))
}

// GetUserActivities returns activities for a specific user (admin only)
func (h *ActivityHandler) GetUserActivities(c *gin.Context) {
	userID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user ID"})
		return
	}

	limit := 100 // Default for admin view
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	activities, err := h.activityService.GetUserActivities(c.Request.Context(), uint(userID), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get activities"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user_id":    userID,
		"activities": domain.ToUserActivityDTOs(activities),
		"count":      len(activities),
	})
}

// ListActivities returns all activities with filters (admin only)
func (h *ActivityHandler) ListActivities(c *gin.Context) {
	filter := repository.ActivityFilter{
		Limit:  50,
		Offset: 0,
	}

	// Parse filters
	if userIDStr := c.Query("user_id"); userIDStr != "" {
		if uid, err := strconv.ParseUint(userIDStr, 10, 32); err == nil {
			userID := uint(uid)
			filter.UserID = &userID
		}
	}

	if activityType := c.Query("type"); activityType != "" {
		at := domain.ActivityType(activityType)
		filter.ActivityType = &at
	}

	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			filter.Limit = l
		}
	}

	if offsetStr := c.Query("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			filter.Offset = o
		}
	}

	activities, total, err := h.activityService.ListActivities(c.Request.Context(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list activities"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"activities": domain.ToUserActivityDTOs(activities),
		"total":      total,
		"limit":      filter.Limit,
		"offset":     filter.Offset,
	})
}
