package http

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/service"
)

type entitlementMembershipReader interface {
	GetActiveByOrgAndUser(ctx context.Context, orgID, userID uint) (*domain.OrganizationUser, error)
}

type EntitlementHandler struct {
	entitlements service.OrganizationEntitlementService
	memberships  entitlementMembershipReader
}

func NewEntitlementHandler(
	entitlements service.OrganizationEntitlementService,
	memberships entitlementMembershipReader,
) *EntitlementHandler {
	return &EntitlementHandler{entitlements: entitlements, memberships: memberships}
}

func (h *EntitlementHandler) Get(c *gin.Context) {
	orgID, ok := GetResolvedOrgID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid organization id"})
		return
	}
	userID := GetCurrentUserID(c)
	if _, err := h.memberships.GetActiveByOrgAndUser(c.Request.Context(), orgID, userID); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
		return
	}

	c.Header("Cache-Control", "no-store")
	snapshot, err := h.entitlements.Resolve(c.Request.Context(), orgID)
	if errors.Is(err, service.ErrEntitlementSubscriptionUnavailable) {
		c.JSON(http.StatusConflict, gin.H{
			"error": "organization subscription unavailable",
			"code":  "SUBSCRIPTION_UNAVAILABLE",
		})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve entitlements"})
		return
	}
	c.JSON(http.StatusOK, snapshot)
}
