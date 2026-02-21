package http

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	"github.com/passwall/passwall-server/internal/service"
)

type OrganizationHandler struct {
	service service.OrganizationService
	subRepo interface {
		GetByOrganizationID(ctx context.Context, orgID uint) (*domain.Subscription, error)
	}
}

func NewOrganizationHandler(
	service service.OrganizationService,
	subRepo interface {
		GetByOrganizationID(ctx context.Context, orgID uint) (*domain.Subscription, error)
	},
) *OrganizationHandler {
	return &OrganizationHandler{service: service, subRepo: subRepo}
}

// Create godoc
// @Summary Create organization
// @Description Create a new organization
// @Tags organizations
// @Accept json
// @Produce json
// @Param request body domain.CreateOrganizationRequest true "Organization details"
// @Success 201 {object} domain.OrganizationDTO
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /organizations [post]
func (h *OrganizationHandler) Create(c *gin.Context) {
	ctx := c.Request.Context()
	userID := GetCurrentUserID(c)

	var req domain.CreateOrganizationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "details": err.Error()})
		return
	}

	org, err := h.service.Create(ctx, userID, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create organization", "details": err.Error()})
		return
	}

	sub, _ := h.subRepo.GetByOrganizationID(ctx, org.ID)
	c.JSON(http.StatusCreated, domain.ToOrganizationDTOWithSubscription(org, sub))
}

// List godoc
// @Summary List organizations
// @Description List organizations for current user
// @Tags organizations
// @Produce json
// @Success 200 {array} domain.OrganizationDTO
// @Failure 500 {object} map[string]string
// @Router /organizations [get]
func (h *OrganizationHandler) List(c *gin.Context) {
	ctx := c.Request.Context()
	userID := GetCurrentUserID(c)

	orgs, err := h.service.List(ctx, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list organizations"})
		return
	}

	dtos := make([]*domain.OrganizationDTO, len(orgs))
	for i, org := range orgs {
		sub, _ := h.subRepo.GetByOrganizationID(ctx, org.ID)
		dtos[i] = domain.ToOrganizationDTOWithSubscription(org, sub)
	}

	c.JSON(http.StatusOK, dtos)
}

// GetByID godoc
// @Summary Get organization
// @Description Get organization by ID
// @Tags organizations
// @Produce json
// @Param id path int true "Organization ID"
// @Success 200 {object} domain.OrganizationDTO
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /organizations/{id} [get]
func (h *OrganizationHandler) GetByID(c *gin.Context) {
	ctx := c.Request.Context()
	userID := GetCurrentUserID(c)

	id, ok := GetUintParam(c, "id")
	if !ok {
		return
	}

	org, err := h.service.GetByID(ctx, id, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "organization not found"})
			return
		}
		if errors.Is(err, repository.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get organization"})
		return
	}

	sub, _ := h.subRepo.GetByOrganizationID(ctx, org.ID)
	c.JSON(http.StatusOK, domain.ToOrganizationDTOWithSubscription(org, sub))
}

// Update godoc
// @Summary Update organization
// @Description Update organization details
// @Tags organizations
// @Accept json
// @Produce json
// @Param id path int true "Organization ID"
// @Param request body domain.UpdateOrganizationRequest true "Organization details"
// @Success 200 {object} domain.OrganizationDTO
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /organizations/{id} [put]
func (h *OrganizationHandler) Update(c *gin.Context) {
	ctx := c.Request.Context()
	userID := GetCurrentUserID(c)

	id, ok := GetUintParam(c, "id")
	if !ok {
		return
	}

	var req domain.UpdateOrganizationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "details": err.Error()})
		return
	}

	org, err := h.service.Update(ctx, id, userID, &req)
	if err != nil {
		if errors.Is(err, repository.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
			return
		}
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "organization not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update organization", "details": err.Error()})
		return
	}

	sub, _ := h.subRepo.GetByOrganizationID(ctx, org.ID)
	c.JSON(http.StatusOK, domain.ToOrganizationDTOWithSubscription(org, sub))
}

// Delete godoc
// @Summary Delete organization
// @Description Delete organization (owner only)
// @Tags organizations
// @Param id path int true "Organization ID"
// @Success 204
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /organizations/{id} [delete]
func (h *OrganizationHandler) Delete(c *gin.Context) {
	ctx := c.Request.Context()
	userID := GetCurrentUserID(c)

	id, ok := GetUintParam(c, "id")
	if !ok {
		return
	}

	err := h.service.Delete(ctx, id, userID)
	if err != nil {
		if errors.Is(err, repository.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "only owner can delete organization"})
			return
		}
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "organization not found"})
			return
		}
		// Validation/business-rule errors (e.g. default org cannot be deleted)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// InviteUser godoc
// @Summary Invite user to organization
// @Description Invite a user to join the organization
// @Tags organizations
// @Accept json
// @Produce json
// @Param id path int true "Organization ID"
// @Param request body domain.InviteUserToOrgRequest true "Invitation details"
// @Success 201 {object} domain.OrganizationUserDTO
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Router /organizations/{id}/members [post]
func (h *OrganizationHandler) InviteUser(c *gin.Context) {
	ctx := c.Request.Context()
	userID := GetCurrentUserID(c)

	orgID, ok := GetUintParam(c, "id")
	if !ok {
		return
	}

	var req domain.InviteUserToOrgRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "details": err.Error()})
		return
	}

	orgUser, err := h.service.InviteUser(ctx, orgID, userID, &req)
	if err != nil {
		if errors.Is(err, repository.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to invite user", "details": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, domain.ToOrganizationUserDTO(orgUser))
}

// GetMembers godoc
// @Summary Get organization members
// @Description Get list of organization members
// @Tags organizations
// @Produce json
// @Param id path int true "Organization ID"
// @Success 200 {array} domain.OrganizationUserDTO
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Router /organizations/{id}/members [get]
func (h *OrganizationHandler) GetMembers(c *gin.Context) {
	ctx := c.Request.Context()
	userID := GetCurrentUserID(c)

	orgID, ok := GetUintParam(c, "id")
	if !ok {
		return
	}

	members, err := h.service.GetMembers(ctx, orgID, userID)
	if err != nil {
		if errors.Is(err, repository.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get members"})
		return
	}

	dtos := make([]*domain.OrganizationUserDTO, len(members))
	for i, m := range members {
		dtos[i] = domain.ToOrganizationUserDTO(m)
	}

	c.JSON(http.StatusOK, dtos)
}

// UpdateMemberRole godoc
// @Summary Update member role
// @Description Update organization member role
// @Tags organizations
// @Accept json
// @Produce json
// @Param id path int true "Organization ID"
// @Param userId path int true "Organization User ID"
// @Param request body domain.UpdateOrgUserRoleRequest true "Role update"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Router /organizations/{id}/members/{userId} [put]
func (h *OrganizationHandler) UpdateMemberRole(c *gin.Context) {
	ctx := c.Request.Context()
	userID := GetCurrentUserID(c)

	orgID, ok := GetUintParam(c, "id")
	if !ok {
		return
	}

	orgUserID, ok := GetUintParam(c, "userId")
	if !ok {
		return
	}

	var req domain.UpdateOrgUserRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "details": err.Error()})
		return
	}

	err := h.service.UpdateMemberRole(ctx, orgID, orgUserID, userID, &req)
	if err != nil {
		if errors.Is(err, repository.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to update member role", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "member role updated successfully"})
}

// RemoveMember godoc
// @Summary Remove member from organization
// @Description Remove a member from the organization
// @Tags organizations
// @Param id path int true "Organization ID"
// @Param userId path int true "Organization User ID"
// @Success 204
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Router /organizations/{id}/members/{userId} [delete]
func (h *OrganizationHandler) RemoveMember(c *gin.Context) {
	ctx := c.Request.Context()
	userID := GetCurrentUserID(c)

	orgID, ok := GetUintParam(c, "id")
	if !ok {
		return
	}

	orgUserID, ok := GetUintParam(c, "userId")
	if !ok {
		return
	}

	err := h.service.RemoveMember(ctx, orgID, orgUserID, userID)
	if err != nil {
		if errors.Is(err, repository.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to remove member", "details": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// AcceptInvitation godoc
// @Summary Accept organization invitation
// @Description Accept an invitation to join an organization
// @Tags organizations
// @Param id path int true "Organization User ID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Router /organizations/invitations/{id}/accept [post]
func (h *OrganizationHandler) AcceptInvitation(c *gin.Context) {
	ctx := c.Request.Context()
	userID := GetCurrentUserID(c)

	var req struct {
		EncryptedOrgKey string `json:"encrypted_org_key" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "details": err.Error()})
		return
	}

	orgUserID, ok := GetUintParam(c, "id")
	if !ok {
		return
	}

	err := h.service.AcceptInvitation(ctx, orgUserID, userID, req.EncryptedOrgKey)
	if err != nil {
		if errors.Is(err, repository.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "not authorized to accept this invitation"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to accept invitation", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "invitation accepted successfully"})
}
