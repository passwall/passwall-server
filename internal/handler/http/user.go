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

type UserHandler struct {
	service service.UserService
}

func NewUserHandler(service service.UserService) *UserHandler {
	return &UserHandler{service: service}
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

	var req domain.CreateUserByAdminRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "details": err.Error()})
		return
	}

	// Set default role if not provided
	if req.RoleID == nil {
		defaultRole := constants.RoleIDMember
		req.RoleID = &defaultRole
	}

	user, err := h.service.CreateByAdmin(ctx, &req)
	if err != nil {
		if errors.Is(err, repository.ErrAlreadyExists) {
			c.JSON(http.StatusConflict, gin.H{"error": "user already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create user", "details": err.Error()})
		return
	}

	// Get created user with role data
	createdUser, err := h.service.GetByID(ctx, user.ID)
	if err == nil {
		c.JSON(http.StatusCreated, domain.ToUserDTO(createdUser))
		return
	}

	c.JSON(http.StatusCreated, domain.ToUserDTO(user))
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

// GetPublicKey godoc
// @Summary Get user's RSA public key
// @Description Get user's RSA public key by email (for organization key wrapping)
// @Tags users
// @Produce json
// @Param email query string true "User email"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /users/public-key [get]
func (h *UserHandler) GetPublicKey(c *gin.Context) {
	ctx := c.Request.Context()

	email := c.Query("email")
	if email == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "email parameter is required"})
		return
	}

	user, err := h.service.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get user"})
		return
	}

	// Return public key (can be null if user hasn't joined an org yet)
	response := gin.H{
		"user_id":        user.ID,
		"email":          user.Email,
		"rsa_public_key": user.RSAPublicKey,
	}

	c.JSON(http.StatusOK, response)
}

// CheckRSAKeys godoc
// @Summary Check if user has RSA keys
// @Description Check if current user has RSA keys generated
// @Tags users
// @Produce json
// @Success 200 {object} map[string]bool
// @Router /users/me/rsa-keys [get]
func (h *UserHandler) CheckRSAKeys(c *gin.Context) {
	ctx := c.Request.Context()
	userID := GetCurrentUserID(c)

	user, err := h.service.GetByID(ctx, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get user"})
		return
	}

	hasKeys := user.RSAPublicKey != nil && *user.RSAPublicKey != ""

	c.JSON(http.StatusOK, gin.H{"has_rsa_keys": hasKeys})
}

// StoreRSAKeys godoc
// @Summary Store user's RSA keys
// @Description Store user's RSA key pair (generated client-side)
// @Tags users
// @Accept json
// @Produce json
// @Param request body map[string]string true "RSA keys"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Router /users/me/rsa-keys [post]
func (h *UserHandler) StoreRSAKeys(c *gin.Context) {
	ctx := c.Request.Context()
	userID := GetCurrentUserID(c)

	var req struct {
		RSAPublicKey     string `json:"rsa_public_key" binding:"required"`
		RSAPrivateKeyEnc string `json:"rsa_private_key_enc" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "details": err.Error()})
		return
	}

	// Get user
	user, err := h.service.GetByID(ctx, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get user"})
		return
	}

	// Update RSA keys
	user.RSAPublicKey = &req.RSAPublicKey
	user.RSAPrivateKeyEnc = &req.RSAPrivateKeyEnc

	if err := h.service.Update(ctx, userID, user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to store RSA keys"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "RSA keys stored successfully"})
}

// CheckOwnership checks if user is sole owner of any organizations
func (h *UserHandler) CheckOwnership(c *gin.Context) {
	ctx := c.Request.Context()
	userID, ok := GetUintParam(c, "id")
	if !ok {
		return
	}

	// Verify requester is admin or the user themselves
	currentUserID := GetCurrentUserID(c)
	currentRole, _ := c.Get(constants.ContextKeyUserRole)
	if currentUserID != userID && currentRole != constants.RoleAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	result, err := h.service.CheckOwnership(ctx, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check ownership"})
		return
	}

	c.JSON(http.StatusOK, result)
}

// TransferOwnership transfers organization ownership to another user
func (h *UserHandler) TransferOwnership(c *gin.Context) {
	ctx := c.Request.Context()
	userID, ok := GetUintParam(c, "id")
	if !ok {
		return
	}

	// Verify requester is admin or the user themselves
	currentUserID := GetCurrentUserID(c)
	currentRole, _ := c.Get(constants.ContextKeyUserRole)
	if currentUserID != userID && currentRole != constants.RoleAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	var req domain.TransferOwnershipRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "details": err.Error()})
		return
	}

	// Set user ID from URL param
	req.UserID = userID

	if err := h.service.TransferOwnership(ctx, &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "ownership transferred successfully"})
}

// DeleteWithOrganizations deletes user along with their sole-owner organizations
func (h *UserHandler) DeleteWithOrganizations(c *gin.Context) {
	ctx := c.Request.Context()
	userID, ok := GetUintParam(c, "id")
	if !ok {
		return
	}

	// Only admins can delete users
	currentRole, _ := c.Get(constants.ContextKeyUserRole)
	if currentRole != constants.RoleAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	var req domain.DeleteWithOrganizationsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "details": err.Error()})
		return
	}

	// Get user to extract schema
	user, err := h.service.GetByID(ctx, userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	if err := h.service.DeleteWithOrganizations(ctx, userID, req.OrganizationIDs, user.Schema); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "user and organizations deleted successfully"})
}
