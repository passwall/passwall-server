package http

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/email"
	"github.com/passwall/passwall-server/internal/repository"
	"github.com/passwall/passwall-server/internal/service"
	"github.com/passwall/passwall-server/pkg/constants"
)

type UserHandler struct {
	service     service.UserService
	emailSender email.Sender
}

func NewUserHandler(service service.UserService, emailSender email.Sender) *UserHandler {
	return &UserHandler{service: service, emailSender: emailSender}
}

func (h *UserHandler) List(c *gin.Context) {
	ctx := c.Request.Context()

	users, err := h.service.List(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch users"})
		return
	}

	// Convert to DTOs for API response
	c.JSON(http.StatusOK, domain.ToUserDTOs(users))
}

func (h *UserHandler) Create(c *gin.Context) {
	ctx := c.Request.Context()

	var req domain.AdminCreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "details": err.Error()})
		return
	}

	// Default role if not provided
	if req.RoleID == 0 {
		req.RoleID = constants.RoleIDMember
	}

	createdUser, err := h.service.CreateAdmin(ctx, &req)
	if err != nil {
		if errors.Is(err, repository.ErrAlreadyExists) {
			c.JSON(http.StatusConflict, gin.H{"error": "user already exists"})
			return
		}
		if errors.Is(err, repository.ErrInvalidInput) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid input"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create user"})
		return
	}

	c.JSON(http.StatusCreated, domain.ToUserDTO(createdUser))
}

func (h *UserHandler) Invite(c *gin.Context) {
	ctx := c.Request.Context()

	var req domain.AdminInviteUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "details": err.Error()})
		return
	}

	// Send invitation email (does not create user)
	if err := h.emailSender.SendInviteEmail(ctx, req.Email, req.Role, req.Desc); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to send invite email"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *UserHandler) GetByID(c *gin.Context) {
	ctx := c.Request.Context()

	id, ok := GetUintParam(c, "id")
	if !ok {
		return
	}

	user, err := h.service.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch user"})
		return
	}

	// Convert to DTO for API response
	c.JSON(http.StatusOK, domain.ToUserDTO(user))
}

func (h *UserHandler) Update(c *gin.Context) {
	ctx := c.Request.Context()

	id, ok := GetUintParam(c, "id")
	if !ok {
		return
	}

	var req domain.UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "details": err.Error()})
		return
	}

	// Check if there are any updates
	if !req.HasUpdates() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no fields to update"})
		return
	}

	// Get existing user
	existingUser, err := h.service.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch user"})
		return
	}

	// Apply updates
	req.ApplyTo(existingUser)

	// Update in database
	if err := h.service.Update(ctx, id, existingUser); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update user"})
		return
	}

	// Get updated user with fresh role data
	updatedUser, err := h.service.GetByID(ctx, id)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "user updated successfully"})
		return
	}

	// Convert to DTO for API response
	c.JSON(http.StatusOK, domain.ToUserDTO(updatedUser))
}

func (h *UserHandler) Delete(c *gin.Context) {
	ctx := c.Request.Context()

	id, ok := GetUintParam(c, "id")
	if !ok {
		return
	}

	// Get user's schema before deletion
	user, err := h.service.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch user"})
		return
	}

	if err := h.service.Delete(ctx, id, user.Schema); err != nil {
		if errors.Is(err, repository.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "cannot delete system user"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete user"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "user deleted successfully"})
}

func (h *UserHandler) UpdateProfile(c *gin.Context) {
	ctx := c.Request.Context()

	// Get authenticated user ID from context (set by AuthMiddleware)
	userIDValue, exists := c.Get(constants.ContextKeyUserID)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return
	}

	// Type assertion with safety check
	userID, ok := userIDValue.(uint)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user id"})
		return
	}

	var req domain.UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "details": err.Error()})
		return
	}

	// Prevent role_id changes via profile update (security)
	if req.RoleID != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "cannot change role via profile update"})
		return
	}

	// Check if there are any updates
	if !req.HasUpdates() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no fields to update"})
		return
	}

	// Get existing user
	existingUser, err := h.service.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch user"})
		return
	}

	// Apply updates (excluding role_id)
	req.ApplyTo(existingUser)

	// Update in database
	if err := h.service.Update(ctx, userID, existingUser); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update profile"})
		return
	}

	// Get updated user with fresh role data
	updatedUser, err := h.service.GetByID(ctx, userID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "profile updated successfully"})
		return
	}

	// Convert to DTO for API response
	c.JSON(http.StatusOK, domain.ToUserDTO(updatedUser))
}

func (h *UserHandler) ChangeMasterPassword(c *gin.Context) {
	ctx := c.Request.Context()

	var req domain.ChangeMasterPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "details": err.Error()})
		return
	}

	if err := h.service.ChangeMasterPassword(ctx, &req); err != nil {
		if errors.Is(err, repository.ErrUnauthorized) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid old password"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to change password"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "master password changed successfully"})
}
