package http

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	"github.com/passwall/passwall-server/internal/service"
	"github.com/passwall/passwall-server/pkg/constants"
)

type InvitationHandler struct {
	invitationService service.InvitationService
	userService       service.UserService
}

// NewInvitationHandler creates a new invitation handler
func NewInvitationHandler(
	invitationService service.InvitationService,
	userService service.UserService,
) *InvitationHandler {
	return &InvitationHandler{
		invitationService: invitationService,
		userService:       userService,
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
