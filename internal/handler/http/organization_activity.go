package http

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	"github.com/passwall/passwall-server/internal/service"
)

type OrganizationActivityHandler struct {
	activityService service.UserActivityService
	orgUserRepo     repository.OrganizationUserRepository
}

func NewOrganizationActivityHandler(
	activityService service.UserActivityService,
	orgUserRepo repository.OrganizationUserRepository,
) *OrganizationActivityHandler {
	return &OrganizationActivityHandler{
		activityService: activityService,
		orgUserRepo:     orgUserRepo,
	}
}

func parseOrgIDFromDetails(details string) (uint, bool) {
	if details == "" {
		return 0, false
	}
	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(details), &obj); err != nil {
		return 0, false
	}
	raw, ok := obj["organization_id"]
	if !ok {
		return 0, false
	}

	switch v := raw.(type) {
	case float64:
		if v <= 0 {
			return 0, false
		}
		return uint(v), true
	case string:
		n, err := strconv.ParseUint(v, 10, 64)
		if err != nil || n == 0 {
			return 0, false
		}
		return uint(n), true
	default:
		return 0, false
	}
}

// ListOrganizationActivities returns recent activities related to an organization.
// Visibility: any organization member can view.
// Note: Activities are stored per-user; we aggregate members' activities and filter by details.organization_id.
func (h *OrganizationActivityHandler) ListOrganizationActivities(c *gin.Context) {
	ctx := c.Request.Context()
	userID := GetCurrentUserID(c)

	orgID, ok := GetUintParam(c, "id")
	if !ok {
		return
	}

	// Membership check
	if _, err := h.orgUserRepo.GetByOrgAndUser(ctx, orgID, userID); err != nil {
		if err == repository.ErrNotFound {
			c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check membership"})
		return
	}

	limit := 10
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}
	offset := 0
	if offsetStr := c.Query("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}
	typeFilter := c.Query("type")
	var userIDFilter *uint
	if userIDStr := c.Query("user_id"); userIDStr != "" {
		if uid, err := strconv.ParseUint(userIDStr, 10, 32); err == nil && uid > 0 {
			u := uint(uid)
			userIDFilter = &u
		}
	}

	// Collect org member user IDs
	orgUsers, err := h.orgUserRepo.ListByOrganization(ctx, orgID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load organization members"})
		return
	}

	userIDs := make([]uint, 0, len(orgUsers))
	for _, ou := range orgUsers {
		userIDs = append(userIDs, ou.UserID)
	}

	// If a specific user_id is requested, ensure it belongs to this organization.
	if userIDFilter != nil {
		found := false
		for _, id := range userIDs {
			if id == *userIDFilter {
				found = true
				break
			}
		}
		if !found {
			c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
			return
		}
		userIDs = []uint{*userIDFilter}
	}

	// Overfetch to compensate for filtering by details.organization_id.
	// We only need (offset + limit + 1) matching rows to compute has_more.
	targetNeed := offset + limit + 1
	fetchLimit := targetNeed * 10
	if fetchLimit > 1000 {
		fetchLimit = 1000
	}

	activities, err := h.activityService.ListActivitiesByUserIDs(ctx, userIDs, fetchLimit, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list activities"})
		return
	}

	matches := make([]*domain.UserActivityDTO, 0, targetNeed)
	for _, a := range activities {
		if a == nil {
			continue
		}
		if typeFilter != "" && string(a.ActivityType) != typeFilter {
			continue
		}
		if oid, ok := parseOrgIDFromDetails(a.Details); ok && oid == orgID {
			matches = append(matches, domain.ToUserActivityDTO(a))
			if len(matches) >= targetNeed {
				break
			}
		}
	}

	hasMore := len(matches) > offset+limit
	start := offset
	if start > len(matches) {
		start = len(matches)
	}
	end := offset + limit
	if end > len(matches) {
		end = len(matches)
	}
	paged := matches[start:end]

	c.JSON(http.StatusOK, gin.H{
		"activities": paged,
		"count":      len(paged),
		"limit":      limit,
		"offset":     offset,
		"has_more":   hasMore,
	})
}
