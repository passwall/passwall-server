package http

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/service"
	"github.com/passwall/passwall-server/pkg/logger"
)

// KeyEscrowHandler handles key escrow HTTP endpoints
type KeyEscrowHandler struct {
	escrowService service.KeyEscrowService
	orgService    interface {
		GetMembership(ctx context.Context, userID uint, orgID uint) (*domain.OrganizationUser, error)
	}
}

// NewKeyEscrowHandler creates a new key escrow handler
func NewKeyEscrowHandler(
	escrowService service.KeyEscrowService,
	orgService interface {
		GetMembership(ctx context.Context, userID uint, orgID uint) (*domain.OrganizationUser, error)
	},
) *KeyEscrowHandler {
	return &KeyEscrowHandler{
		escrowService: escrowService,
		orgService:    orgService,
	}
}

// Enroll accepts the user's raw User Key and wraps it for escrow.
// Called once when a user opts into SSO key escrow (one-time migration).
// POST /api/organizations/:id/key-escrow/enroll
func (h *KeyEscrowHandler) Enroll(c *gin.Context) {
	ctx := c.Request.Context()
	userID := GetCurrentUserID(c)

	orgID, ok := GetUintParam(c, "id")
	if !ok {
		return
	}

	if !h.ensureOrgMember(c, ctx, userID, orgID) {
		return
	}

	var req domain.EnrollKeyEscrowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "details": err.Error()})
		return
	}

	if err := h.escrowService.EnrollUser(ctx, userID, orgID, req.OrgKey); err != nil {
		logger.Errorf("key escrow enroll failed: user_id=%d org_id=%d err=%v", userID, orgID, err)
		if errors.Is(err, service.ErrEscrowNotConfigured) {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "key escrow is not configured on this server"})
			return
		}
		if errors.Is(err, service.ErrEscrowAlreadyExists) {
			c.JSON(http.StatusConflict, gin.H{"error": "key escrow already enrolled for this organization"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to enroll key escrow"})
		return
	}

	logger.Infof("key escrow enrolled: user_id=%d org_id=%d", userID, orgID)
	c.JSON(http.StatusCreated, gin.H{"status": "enrolled"})
}

// GetStatus returns the key escrow status for the current user in an organization.
// GET /api/organizations/:id/key-escrow/status
func (h *KeyEscrowHandler) GetStatus(c *gin.Context) {
	ctx := c.Request.Context()
	userID := GetCurrentUserID(c)

	orgID, ok := GetUintParam(c, "id")
	if !ok {
		return
	}

	if !h.ensureOrgMember(c, ctx, userID, orgID) {
		return
	}

	status, err := h.escrowService.GetStatus(ctx, userID, orgID)
	if err != nil {
		logger.Errorf("key escrow get status failed: user_id=%d org_id=%d err=%v", userID, orgID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get key escrow status"})
		return
	}

	c.JSON(http.StatusOK, status)
}

// Revoke removes a user's escrowed key (admin offboarding or user opt-out).
// DELETE /api/organizations/:id/key-escrow/users/:userId
func (h *KeyEscrowHandler) Revoke(c *gin.Context) {
	ctx := c.Request.Context()
	callerID := GetCurrentUserID(c)

	orgID, ok := GetUintParam(c, "id")
	if !ok {
		return
	}

	targetUserID, ok := GetUintParam(c, "userId")
	if !ok {
		return
	}

	// Allow: admin revoking any user, or user revoking themselves
	if callerID != targetUserID {
		if !h.ensureOrgAdmin(c, ctx, callerID, orgID) {
			return
		}
	} else {
		if !h.ensureOrgMember(c, ctx, callerID, orgID) {
			return
		}
	}

	if err := h.escrowService.RevokeUser(ctx, targetUserID, orgID); err != nil {
		logger.Errorf("key escrow revoke failed: caller_id=%d target_user_id=%d org_id=%d err=%v", callerID, targetUserID, orgID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to revoke key escrow"})
		return
	}

	logger.Infof("key escrow revoked: caller_id=%d target_user_id=%d org_id=%d", callerID, targetUserID, orgID)
	c.JSON(http.StatusNoContent, nil)
}

func (h *KeyEscrowHandler) ensureOrgMember(c *gin.Context, ctx context.Context, userID, orgID uint) bool {
	membership, err := h.orgService.GetMembership(ctx, userID, orgID)
	if err != nil || membership == nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "organization access denied"})
		return false
	}
	return true
}

func (h *KeyEscrowHandler) ensureOrgAdmin(c *gin.Context, ctx context.Context, userID, orgID uint) bool {
	membership, err := h.orgService.GetMembership(ctx, userID, orgID)
	if err != nil || membership == nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "organization access denied"})
		return false
	}
	if !membership.IsAdmin() {
		c.JSON(http.StatusForbidden, gin.H{"error": "organization admin access required"})
		return false
	}
	return true
}
