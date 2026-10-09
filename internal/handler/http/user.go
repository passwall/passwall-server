package http

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	"github.com/passwall/passwall-server/internal/service"
	"github.com/passwall/passwall-server/pkg/constants"
)

type UserHandler struct {
	service         service.UserService
	activityService service.UserActivityService
	activityLogger  *service.ActivityLogger
	lastSignIns     lastSignInReader
}

type lastSignInReader interface {
	GetLastSignInTimes(ctx context.Context, userIDs []uint) (map[uint]time.Time, error)
}

func NewUserHandler(userService service.UserService, activityService service.UserActivityService, lastSignIns lastSignInReader) *UserHandler {
	return &UserHandler{
		service:         userService,
		activityService: activityService,
		activityLogger:  service.NewActivityLogger(activityService),
		lastSignIns:     lastSignIns,
	}
}

type adminUserListResponse struct {
	Items    []*domain.UserDTO `json:"items"`
	Total    int64             `json:"total"`
	Filtered int64             `json:"filtered"`
}

// List users for platform admins, paginated server-side.
// GET /api/users?search=&limit=&offset=
func (h *UserHandler) List(c *gin.Context) {
	ctx := c.Request.Context()
	limit, offset := parseAdminPagination(c)

	users, result, err := h.service.ListPage(ctx, repository.ListFilter{
		Search: strings.TrimSpace(c.Query("search")),
		Limit:  limit,
		Offset: offset,
		Sort:   "created_at",
		Order:  "desc",
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch users"})
		return
	}

	dtos := domain.ToUserDTOs(users)
	if h.lastSignIns != nil && len(users) > 0 {
		ids := make([]uint, len(users))
		for i, u := range users {
			ids[i] = u.ID
		}
		if times, err := h.lastSignIns.GetLastSignInTimes(ctx, ids); err == nil {
			for i, u := range users {
				if at, ok := times[u.ID]; ok {
					at := at
					dtos[i].LastSignInAt = &at
				}
			}
		}
	}
	c.JSON(http.StatusOK, adminUserListResponse{Items: dtos, Total: result.Total, Filtered: result.Filtered})
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
	actorID, err := GetUserID(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	var req domain.UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "details": err.Error()})
		return
	}
	if !req.HasUpdates() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no fields to update"})
		return
	}

	before, updated, err := h.service.UpdateByAdmin(ctx, actorID, id, &req)
	if err != nil {
		writeAdminUserError(c, err, "failed to update user")
		return
	}
	h.logUserUpdate(c, actorID, before, updated)

	if fresh, err := h.service.GetByID(ctx, id); err == nil {
		updated = fresh
	}
	c.JSON(http.StatusOK, domain.ToUserDTO(updated))
}

// logUserUpdate writes the audit entry synchronously; a role change gets its
// own activity type so it is easy to find.
func (h *UserHandler) logUserUpdate(c *gin.Context, actorID uint, before domain.User, after *domain.User) {
	if h.activityLogger == nil || after == nil {
		return
	}
	details := service.ActivityDetails{
		service.ActivityFieldUserID:    after.ID,
		service.ActivityFieldUserEmail: after.Email,
	}
	if before.Name != after.Name {
		details["old_name"], details["new_name"] = before.Name, after.Name
	}
	if before.Email != after.Email {
		details["old_email"], details["new_email"] = before.Email, after.Email
	}
	if before.Language != after.Language {
		details["old_language"], details["new_language"] = before.Language, after.Language
	}
	activityType := domain.ActivityTypeAdminUserUpdated
	if before.RoleID != after.RoleID {
		details[service.ActivityFieldOldRole], details[service.ActivityFieldNewRole] = before.RoleID, after.RoleID
		activityType = domain.ActivityTypeAdminUserRoleChanged
	}
	_ = h.activityLogger.LogActivity(c.Request.Context(), actorID, activityType, GetIPAddress(c), GetUserAgent(c), details)
}

func writeAdminUserError(c *gin.Context, err error, fallback string) {
	type mapped struct {
		status int
		code   string
	}
	for target, m := range map[error]mapped{
		service.ErrAdminRoleInvalid:    {http.StatusBadRequest, "ROLE_INVALID"},
		service.ErrAdminSelfRoleChange: {http.StatusBadRequest, "CANNOT_CHANGE_OWN_ROLE"},
		service.ErrAdminSelfDelete:     {http.StatusBadRequest, "CANNOT_DELETE_SELF"},
		service.ErrSystemUserProtected: {http.StatusConflict, "SYSTEM_USER_PROTECTED"},
		service.ErrLastAdmin:           {http.StatusConflict, "LAST_ADMIN"},
	} {
		if errors.Is(err, target) {
			c.JSON(m.status, gin.H{"error": target.Error(), "code": m.code})
			return
		}
	}
	if errors.Is(err, repository.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": fallback})
}

func (h *UserHandler) Delete(c *gin.Context) {
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

	actorID, err := GetUserID(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	if actorID == id {
		writeAdminUserError(c, service.ErrAdminSelfDelete, "failed to delete user")
		return
	}

	if err := h.service.Delete(ctx, id); err != nil {
		if errors.Is(err, repository.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "system users cannot be deleted", "code": "SYSTEM_USER_PROTECTED"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to delete user", "details": err.Error()})
		return
	}

	// Audit after the delete succeeded so failed attempts are not recorded as deletions.
	if h.activityLogger != nil {
		_ = h.activityLogger.LogActivity(ctx, actorID, domain.ActivityTypeAdminUserDeleted, GetIPAddress(c), GetUserAgent(c), service.ActivityDetails{
			service.ActivityFieldUserID:    id,
			service.ActivityFieldUserEmail: user.Email,
			service.ActivityFieldRole:      user.RoleID,
		})
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
	email = strings.ToLower(strings.TrimSpace(email))
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

	hasPublic := user.RSAPublicKey != nil && *user.RSAPublicKey != ""
	hasPrivate := user.RSAPrivateKeyEnc != nil && *user.RSAPrivateKeyEnc != ""
	hasKeys := hasPublic && hasPrivate

	c.JSON(http.StatusOK, gin.H{"has_rsa_keys": hasKeys})
}

// GetRSAPrivateKeyEnc returns the encrypted RSA private key for current user
// @Summary Get RSA private key (encrypted)
// @Description Returns user's RSA private key encrypted with User Key
// @Tags users
// @Produce json
// @Success 200 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /users/me/rsa-private-key [get]
func (h *UserHandler) GetRSAPrivateKeyEnc(c *gin.Context) {
	ctx := c.Request.Context()
	userID := GetCurrentUserID(c)

	user, err := h.service.GetByID(ctx, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get user"})
		return
	}

	if user.RSAPrivateKeyEnc == nil || *user.RSAPrivateKeyEnc == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "rsa private key not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"rsa_private_key_enc": *user.RSAPrivateKeyEnc,
	})
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

	hasPublic := user.RSAPublicKey != nil && *user.RSAPublicKey != ""
	hasPrivate := user.RSAPrivateKeyEnc != nil && *user.RSAPrivateKeyEnc != ""

	// Idempotent no-op when both keys are already present.
	if hasPublic && hasPrivate {
		c.JSON(http.StatusOK, gin.H{"message": "RSA keys already configured"})
		return
	}

	// Partial keypair is an inconsistent state. Do not overwrite automatically,
	// because existing organization keys might already be wrapped for this identity.
	if hasPublic != hasPrivate {
		c.JSON(http.StatusConflict, gin.H{
			"error": "rsa keypair is incomplete; manual recovery required",
		})
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

	if err := h.service.DeleteWithOrganizations(ctx, userID, req.OrganizationIDs); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "user and organizations deleted successfully"})
}
