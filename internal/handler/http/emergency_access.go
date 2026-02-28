package http

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	"github.com/passwall/passwall-server/internal/service"
)

type EmergencyAccessHandler struct {
	service  service.EmergencyAccessService
	userRepo repository.UserRepository
}

func NewEmergencyAccessHandler(svc service.EmergencyAccessService, userRepo repository.UserRepository) *EmergencyAccessHandler {
	return &EmergencyAccessHandler{service: svc, userRepo: userRepo}
}

// Invite handles POST /api/emergency-access
func (h *EmergencyAccessHandler) Invite(c *gin.Context) {
	ctx := c.Request.Context()
	userID := GetCurrentUserID(c)

	var req struct {
		Email string `json:"email" binding:"required,email"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "valid email is required", "details": err.Error()})
		return
	}

	ea, err := h.service.Invite(ctx, userID, req.Email)
	if err != nil {
		if errors.Is(err, repository.ErrInvalidInput) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: cannot add yourself as emergency contact"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create emergency access invitation"})
		return
	}

	c.JSON(http.StatusCreated, domain.ToEmergencyAccessDTO(ea))
}

// ListGranted handles GET /api/emergency-access/granted
func (h *EmergencyAccessHandler) ListGranted(c *gin.Context) {
	ctx := c.Request.Context()
	userID := GetCurrentUserID(c)

	list, err := h.service.ListGranted(ctx, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list emergency access grants"})
		return
	}

	dtos := make([]*domain.EmergencyAccessDTO, 0, len(list))
	for _, ea := range list {
		dtos = append(dtos, domain.ToEmergencyAccessDTO(ea))
	}

	c.JSON(http.StatusOK, gin.H{"emergency_accesses": dtos})
}

// ListTrusted handles GET /api/emergency-access/trusted
func (h *EmergencyAccessHandler) ListTrusted(c *gin.Context) {
	ctx := c.Request.Context()
	userID := GetCurrentUserID(c)

	user, err := h.userRepo.GetByID(ctx, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get user"})
		return
	}

	list, err := h.service.ListTrusted(ctx, userID, user.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list trusted contacts"})
		return
	}

	dtos := make([]*domain.EmergencyAccessDTO, 0, len(list))
	for _, ea := range list {
		dtos = append(dtos, domain.ToEmergencyAccessDTO(ea))
	}

	c.JSON(http.StatusOK, gin.H{"emergency_accesses": dtos})
}

// Accept handles POST /api/emergency-access/:uuid/accept
func (h *EmergencyAccessHandler) Accept(c *gin.Context) {
	ctx := c.Request.Context()
	userID := GetCurrentUserID(c)

	eaUUID, ok := GetStringParam(c, "uuid")
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "uuid is required"})
		return
	}

	ea, err := h.service.Accept(ctx, userID, eaUUID)
	if err != nil {
		if errors.Is(err, repository.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "this invitation is not for your email address"})
			return
		}
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "emergency access not found"})
			return
		}
		if errors.Is(err, repository.ErrInvalidInput) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invitation is not in a valid state"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to accept invitation"})
		return
	}

	c.JSON(http.StatusOK, domain.ToEmergencyAccessDTO(ea))
}

// Confirm handles POST /api/emergency-access/:uuid/confirm
func (h *EmergencyAccessHandler) Confirm(c *gin.Context) {
	ctx := c.Request.Context()
	userID := GetCurrentUserID(c)

	eaUUID, ok := GetStringParam(c, "uuid")
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "uuid is required"})
		return
	}

	var req struct {
		KeyEncrypted string `json:"key_encrypted" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "key_encrypted is required", "details": err.Error()})
		return
	}

	ea, err := h.service.Confirm(ctx, userID, eaUUID, req.KeyEncrypted)
	if err != nil {
		if errors.Is(err, repository.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
			return
		}
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "emergency access not found"})
			return
		}
		if errors.Is(err, repository.ErrInvalidInput) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "emergency access is not in accepted state"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to confirm emergency access"})
		return
	}

	c.JSON(http.StatusOK, domain.ToEmergencyAccessDTO(ea))
}

// RequestRecovery handles POST /api/emergency-access/:uuid/request
func (h *EmergencyAccessHandler) RequestRecovery(c *gin.Context) {
	ctx := c.Request.Context()
	userID := GetCurrentUserID(c)

	eaUUID, ok := GetStringParam(c, "uuid")
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "uuid is required"})
		return
	}

	ea, err := h.service.RequestRecovery(ctx, userID, eaUUID)
	if err != nil {
		if errors.Is(err, repository.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
			return
		}
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "emergency access not found"})
			return
		}
		if errors.Is(err, repository.ErrInvalidInput) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "emergency access is not confirmed"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to request recovery"})
		return
	}

	c.JSON(http.StatusOK, domain.ToEmergencyAccessDTO(ea))
}

// ApproveRecovery handles POST /api/emergency-access/:uuid/approve
func (h *EmergencyAccessHandler) ApproveRecovery(c *gin.Context) {
	ctx := c.Request.Context()
	userID := GetCurrentUserID(c)

	eaUUID, ok := GetStringParam(c, "uuid")
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "uuid is required"})
		return
	}

	ea, err := h.service.ApproveRecovery(ctx, userID, eaUUID)
	if err != nil {
		if errors.Is(err, repository.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
			return
		}
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "emergency access not found"})
			return
		}
		if errors.Is(err, repository.ErrInvalidInput) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "no pending recovery request"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to approve recovery"})
		return
	}

	c.JSON(http.StatusOK, domain.ToEmergencyAccessDTO(ea))
}

// RejectRecovery handles POST /api/emergency-access/:uuid/reject
func (h *EmergencyAccessHandler) RejectRecovery(c *gin.Context) {
	ctx := c.Request.Context()
	userID := GetCurrentUserID(c)

	eaUUID, ok := GetStringParam(c, "uuid")
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "uuid is required"})
		return
	}

	ea, err := h.service.RejectRecovery(ctx, userID, eaUUID)
	if err != nil {
		if errors.Is(err, repository.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
			return
		}
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "emergency access not found"})
			return
		}
		if errors.Is(err, repository.ErrInvalidInput) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "no pending recovery request"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to reject recovery"})
		return
	}

	c.JSON(http.StatusOK, domain.ToEmergencyAccessDTO(ea))
}

// RevokeAccess handles DELETE /api/emergency-access/:uuid
func (h *EmergencyAccessHandler) RevokeAccess(c *gin.Context) {
	ctx := c.Request.Context()
	userID := GetCurrentUserID(c)

	eaUUID, ok := GetStringParam(c, "uuid")
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "uuid is required"})
		return
	}

	if err := h.service.Revoke(ctx, userID, eaUUID); err != nil {
		if errors.Is(err, repository.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
			return
		}
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "emergency access not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to revoke emergency access"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "emergency access revoked"})
}

// GetVault handles GET /api/emergency-access/:uuid/vault
func (h *EmergencyAccessHandler) GetVault(c *gin.Context) {
	ctx := c.Request.Context()
	userID := GetCurrentUserID(c)

	eaUUID, ok := GetStringParam(c, "uuid")
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "uuid is required"})
		return
	}

	vault, err := h.service.GetVaultForRecovery(ctx, userID, eaUUID)
	if err != nil {
		if errors.Is(err, repository.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "recovery not approved"})
			return
		}
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "emergency access not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get vault"})
		return
	}

	items := make([]gin.H, 0, len(vault.Items))
	for _, item := range vault.Items {
		items = append(items, gin.H{
			"id":              item.ID,
			"uuid":            item.UUID,
			"organization_id": item.OrganizationID,
			"item_type":       item.ItemType,
			"data":            item.Data,
			"metadata":        item.Metadata,
			"created_at":      item.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"key_encrypted": vault.KeyEncrypted,
		"items":         items,
	})
}
