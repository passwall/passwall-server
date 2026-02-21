package http

import (
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	"github.com/passwall/passwall-server/internal/service"
	"github.com/passwall/passwall-server/pkg/constants"
)

type InvitationHandler struct {
	invitationService   service.InvitationService
	userService         service.UserService
	organizationService service.OrganizationService
}

// NewInvitationHandler creates a new invitation handler
func NewInvitationHandler(
	invitationService service.InvitationService,
	userService service.UserService,
	organizationService service.OrganizationService,
) *InvitationHandler {
	return &InvitationHandler{
		invitationService:   invitationService,
		userService:         userService,
		organizationService: organizationService,
	}
}

// Invite handles POST /api/invite (any authenticated user)
// Admins can specify role, regular users can only invite as member
func (h *InvitationHandler) Invite(c *gin.Context) {
	ctx := c.Request.Context()

	// Get authenticated user ID and role from context
	userIDValue, exists := c.Get(constants.ContextKeyUserID)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return
	}

	userID, ok := userIDValue.(uint)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user id"})
		return
	}

	// Get user info for inviter name and role check
	user, err := h.userService.GetByID(ctx, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get user info"})
		return
	}

	var req struct {
		Email       string  `json:"email" binding:"required,email"`
		RoleID      *uint   `json:"role_id,omitempty"` // Optional, only admins can specify
		Description *string `json:"description,omitempty"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "details": err.Error()})
		return
	}

	// Determine role: admins can choose, regular users always member
	roleID := uint(2) // Default: member
	if req.RoleID != nil {
		// Only admins can specify custom role
		if !user.IsAdmin() {
			c.JSON(http.StatusForbidden, gin.H{"error": "only admins can specify role"})
			return
		}
		roleID = *req.RoleID
	}

	// Create invitation
	inviteReq := &domain.CreateInvitationRequest{
		Email:       req.Email,
		RoleID:      roleID,
		Description: req.Description,
	}

	invitation, err := h.invitationService.CreateInvitation(ctx, inviteReq, userID, user.Name)
	if err != nil {
		if errors.Is(err, repository.ErrAlreadyExists) {
			c.JSON(http.StatusConflict, gin.H{"error": "user with this email already exists"})
			return
		}
		if err.Error() == "active invitation already exists for this email" {
			c.JSON(http.StatusConflict, gin.H{"error": "active invitation already exists for this email"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to send invitation", "details": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":    "invitation sent successfully",
		"email":      invitation.Email,
		"expires_at": invitation.ExpiresAt,
		"note":       "Invitation email sent",
	})
}

// GetPending godoc
// @Summary Get pending invitations
// @Description Get all pending invitations for current user
// @Tags invitations
// @Produce json
// @Success 200 {array} domain.Invitation
// @Failure 401 {object} map[string]string
// @Router /invitations/pending [get]
func (h *InvitationHandler) GetPending(c *gin.Context) {
	ctx := c.Request.Context()
	userID := GetCurrentUserID(c)

	user, err := h.userService.GetByID(ctx, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get user info"})
		return
	}

	invitations, err := h.invitationService.GetPendingInvitations(ctx, user.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get pending invitations"})
		return
	}

	c.JSON(http.StatusOK, invitations)
}

// GetSent godoc
// @Summary Get sent invitations
// @Description Get all invitations sent by current user
// @Tags invitations
// @Produce json
// @Success 200 {array} domain.Invitation
// @Failure 401 {object} map[string]string
// @Router /invitations/sent [get]
func (h *InvitationHandler) GetSent(c *gin.Context) {
	ctx := c.Request.Context()
	userID := GetCurrentUserID(c)

	invitations, err := h.invitationService.GetSentInvitations(ctx, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get sent invitations"})
		return
	}

	c.JSON(http.StatusOK, invitations)
}

// Accept godoc
// @Summary Accept invitation
// @Description Accept a pending invitation
// @Tags invitations
// @Produce json
// @Param id path int true "Invitation ID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Router /invitations/{id}/accept [post]
func (h *InvitationHandler) Accept(c *gin.Context) {
	ctx := c.Request.Context()
	userID := GetCurrentUserID(c)

	var req struct {
		EncryptedOrgKey string `json:"encrypted_org_key"`
	}
	// Accept empty body for platform invitations, but parse when provided.
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "details": err.Error()})
		return
	}

	invitationID, ok := GetUintParam(c, "id")
	if !ok {
		return
	}

	// Find the specific invitation
	var targetInvitation *domain.Invitation
	user, err := h.userService.GetByID(ctx, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get user info"})
		return
	}

	allInvitations, err := h.invitationService.GetPendingInvitations(ctx, user.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get invitations"})
		return
	}
	for _, inv := range allInvitations {
		if inv.ID == invitationID {
			targetInvitation = inv
			break
		}
	}

	if targetInvitation == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "invitation not found"})
		return
	}

	// Org invitation without wrapped org key cannot be accepted yet.
	// This happens when invitee was not registered (no RSA key) at invite time.
	// Keep invitation pending so owner can resend after invitee completes signup and key generation.
	if targetInvitation.OrganizationID != nil && targetInvitation.OrgRole != nil && targetInvitation.EncryptedOrgKey == nil {
		c.JSON(http.StatusConflict, gin.H{
			"error": "organization invitation needs key provisioning; ask owner to resend invitation after account setup",
		})
		return
	}

	// Accept invitation
	if err := h.invitationService.AcceptInvitation(ctx, invitationID, userID); err != nil {
		if errors.Is(err, repository.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to accept invitation", "details": err.Error()})
		return
	}

	// If this is an organization invitation, add user to org using the invitee-wrapped org key.
	if targetInvitation.OrganizationID != nil && targetInvitation.OrgRole != nil && targetInvitation.EncryptedOrgKey != nil {
		if req.EncryptedOrgKey == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "encrypted_org_key is required for organization invitation acceptance"})
			return
		}

		orgUser := &domain.OrganizationUser{
			OrganizationID:  *targetInvitation.OrganizationID,
			UserID:          userID,
			Role:            domain.OrganizationRole(*targetInvitation.OrgRole),
			EncryptedOrgKey: req.EncryptedOrgKey,
			AccessAll:       targetInvitation.AccessAll,
			Status:          domain.OrgUserStatusAccepted,
		}

		// Use organization service to add user (this is a simplified approach)
		// In production, you might want a dedicated method in org service
		if err := h.organizationService.AddExistingMember(ctx, orgUser); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to join organization", "details": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message":         "invitation accepted and joined organization",
			"organization_id": *targetInvitation.OrganizationID,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "invitation accepted"})
}

// Decline godoc
// @Summary Decline invitation
// @Description Decline a pending invitation
// @Tags invitations
// @Produce json
// @Param id path int true "Invitation ID"
// @Success 200 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Router /invitations/{id}/decline [post]
func (h *InvitationHandler) Decline(c *gin.Context) {
	ctx := c.Request.Context()
	userID := GetCurrentUserID(c)

	invitationID, ok := GetUintParam(c, "id")
	if !ok {
		return
	}

	// Find the specific invitation (needed to clean up org membership invites)
	var targetInvitation *domain.Invitation
	user, err := h.userService.GetByID(ctx, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get user info"})
		return
	}

	allInvitations, err := h.invitationService.GetPendingInvitations(ctx, user.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get invitations"})
		return
	}
	for _, inv := range allInvitations {
		if inv.ID == invitationID {
			targetInvitation = inv
			break
		}
	}

	if targetInvitation == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "invitation not found"})
		return
	}

	// If this is an organization invitation and there is a pending org membership record,
	// remove it so the org doesn't keep a stale "invited" member.
	if targetInvitation.OrganizationID != nil {
		if err := h.organizationService.DeclineInvitationForUser(ctx, *targetInvitation.OrganizationID, userID); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "failed to decline organization invitation", "details": err.Error()})
			return
		}
	}

	if err := h.invitationService.DeclineInvitation(ctx, invitationID, userID); err != nil {
		if errors.Is(err, repository.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to decline invitation", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "invitation declined"})
}
