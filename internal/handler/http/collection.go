package http

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	"github.com/passwall/passwall-server/internal/service"
)

type CollectionHandler struct {
	service service.CollectionService
}

func NewCollectionHandler(service service.CollectionService) *CollectionHandler {
	return &CollectionHandler{service: service}
}

// Create godoc
// @Summary Create collection
// @Description Create a new collection in an organization
// @Tags collections
// @Accept json
// @Produce json
// @Param orgId path int true "Organization ID"
// @Param request body domain.CreateCollectionRequest true "Collection details"
// @Success 201 {object} domain.CollectionDTO
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Router /organizations/{orgId}/collections [post]
func (h *CollectionHandler) Create(c *gin.Context) {
	ctx := c.Request.Context()
	userID := GetCurrentUserID(c)

	orgID, ok := GetUintParam(c, "id")
	if !ok {
		return
	}

	var req domain.CreateCollectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "details": err.Error()})
		return
	}

	collection, err := h.service.Create(ctx, orgID, userID, &req)
	if err != nil {
		if errors.Is(err, repository.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to create collection", "details": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, domain.ToCollectionDTO(collection))
}

// List godoc
// @Summary List collections
// @Description List collections in an organization
// @Tags collections
// @Produce json
// @Param orgId path int true "Organization ID"
// @Success 200 {array} domain.CollectionDTO
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Router /organizations/{orgId}/collections [get]
func (h *CollectionHandler) List(c *gin.Context) {
	ctx := c.Request.Context()
	userID := GetCurrentUserID(c)

	orgID, ok := GetUintParam(c, "id")
	if !ok {
		return
	}

	collections, err := h.service.ListByOrganization(ctx, orgID, userID)
	if err != nil {
		if errors.Is(err, repository.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list collections"})
		return
	}

	dtos := make([]*domain.CollectionDTO, len(collections))
	for i, col := range collections {
		dtos[i] = domain.ToCollectionDTO(col)
	}

	c.JSON(http.StatusOK, dtos)
}

// GetByID godoc
// @Summary Get collection
// @Description Get collection by ID
// @Tags collections
// @Produce json
// @Param id path int true "Collection ID"
// @Success 200 {object} domain.CollectionDTO
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /collections/{id} [get]
func (h *CollectionHandler) GetByID(c *gin.Context) {
	ctx := c.Request.Context()
	userID := GetCurrentUserID(c)

	id, ok := GetUintParam(c, "id")
	if !ok {
		return
	}

	collection, err := h.service.GetByID(ctx, id, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "collection not found"})
			return
		}
		if errors.Is(err, repository.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get collection"})
		return
	}

	c.JSON(http.StatusOK, domain.ToCollectionDTO(collection))
}

// Update godoc
// @Summary Update collection
// @Description Update collection details
// @Tags collections
// @Accept json
// @Produce json
// @Param id path int true "Collection ID"
// @Param request body domain.UpdateCollectionRequest true "Collection details"
// @Success 200 {object} domain.CollectionDTO
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Router /collections/{id} [put]
func (h *CollectionHandler) Update(c *gin.Context) {
	ctx := c.Request.Context()
	userID := GetCurrentUserID(c)

	id, ok := GetUintParam(c, "id")
	if !ok {
		return
	}

	var req domain.UpdateCollectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "details": err.Error()})
		return
	}

	collection, err := h.service.Update(ctx, id, userID, &req)
	if err != nil {
		if errors.Is(err, repository.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to update collection", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, domain.ToCollectionDTO(collection))
}

// Delete godoc
// @Summary Delete collection
// @Description Delete a collection
// @Tags collections
// @Param id path int true "Collection ID"
// @Success 204
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Router /collections/{id} [delete]
func (h *CollectionHandler) Delete(c *gin.Context) {
	ctx := c.Request.Context()
	userID := GetCurrentUserID(c)

	id, ok := GetUintParam(c, "id")
	if !ok {
		return
	}

	err := h.service.Delete(ctx, id, userID)
	if err != nil {
		if errors.Is(err, repository.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete collection"})
		return
	}

	c.Status(http.StatusNoContent)
}

// GrantUserAccess godoc
// @Summary Grant user access to collection
// @Description Grant or update user access to a collection
// @Tags collections
// @Accept json
// @Produce json
// @Param id path int true "Collection ID"
// @Param orgUserId path int true "Organization User ID"
// @Param request body domain.GrantCollectionAccessRequest true "Access details"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Router /collections/{id}/users/{orgUserId} [put]
func (h *CollectionHandler) GrantUserAccess(c *gin.Context) {
	ctx := c.Request.Context()
	userID := GetCurrentUserID(c)

	collectionID, ok := GetUintParam(c, "id")
	if !ok {
		return
	}

	orgUserID, ok := GetUintParam(c, "orgUserId")
	if !ok {
		return
	}

	var req domain.GrantCollectionAccessRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "details": err.Error()})
		return
	}

	err := h.service.GrantUserAccess(ctx, collectionID, orgUserID, userID, &req)
	if err != nil {
		if errors.Is(err, repository.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to grant access", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "access granted successfully"})
}

// GrantTeamAccess godoc
// @Summary Grant team access to collection
// @Description Grant or update team access to a collection
// @Tags collections
// @Accept json
// @Produce json
// @Param id path int true "Collection ID"
// @Param teamId path int true "Team ID"
// @Param request body domain.GrantCollectionAccessRequest true "Access details"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Router /collections/{id}/teams/{teamId} [put]
func (h *CollectionHandler) GrantTeamAccess(c *gin.Context) {
	ctx := c.Request.Context()
	userID := GetCurrentUserID(c)

	collectionID, ok := GetUintParam(c, "id")
	if !ok {
		return
	}

	teamID, ok := GetUintParam(c, "teamId")
	if !ok {
		return
	}

	var req domain.GrantCollectionAccessRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "details": err.Error()})
		return
	}

	err := h.service.GrantTeamAccess(ctx, collectionID, teamID, userID, &req)
	if err != nil {
		if errors.Is(err, repository.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to grant access", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "team access granted successfully"})
}

// RevokeUserAccess godoc
// @Summary Revoke user access
// @Description Revoke user access to a collection
// @Tags collections
// @Param id path int true "Collection ID"
// @Param orgUserId path int true "Organization User ID"
// @Success 204
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Router /collections/{id}/users/{orgUserId} [delete]
func (h *CollectionHandler) RevokeUserAccess(c *gin.Context) {
	ctx := c.Request.Context()
	userID := GetCurrentUserID(c)

	collectionID, ok := GetUintParam(c, "id")
	if !ok {
		return
	}

	orgUserID, ok := GetUintParam(c, "orgUserId")
	if !ok {
		return
	}

	err := h.service.RevokeUserAccess(ctx, collectionID, orgUserID, userID)
	if err != nil {
		if errors.Is(err, repository.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to revoke access"})
		return
	}

	c.Status(http.StatusNoContent)
}

// RevokeTeamAccess godoc
// @Summary Revoke team access
// @Description Revoke team access to a collection
// @Tags collections
// @Param id path int true "Collection ID"
// @Param teamId path int true "Team ID"
// @Success 204
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Router /collections/{id}/teams/{teamId} [delete]
func (h *CollectionHandler) RevokeTeamAccess(c *gin.Context) {
	ctx := c.Request.Context()
	userID := GetCurrentUserID(c)

	collectionID, ok := GetUintParam(c, "id")
	if !ok {
		return
	}

	teamID, ok := GetUintParam(c, "teamId")
	if !ok {
		return
	}

	err := h.service.RevokeTeamAccess(ctx, collectionID, teamID, userID)
	if err != nil {
		if errors.Is(err, repository.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to revoke access"})
		return
	}

	c.Status(http.StatusNoContent)
}

// GetUserAccess godoc
// @Summary Get collection user access
// @Description Get list of users with access to collection
// @Tags collections
// @Produce json
// @Param id path int true "Collection ID"
// @Success 200 {array} domain.CollectionUserDTO
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Router /collections/{id}/users [get]
func (h *CollectionHandler) GetUserAccess(c *gin.Context) {
	ctx := c.Request.Context()
	userID := GetCurrentUserID(c)

	collectionID, ok := GetUintParam(c, "id")
	if !ok {
		return
	}

	users, err := h.service.GetUserAccess(ctx, collectionID, userID)
	if err != nil {
		if errors.Is(err, repository.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get user access"})
		return
	}

	dtos := make([]*domain.CollectionUserDTO, len(users))
	for i, u := range users {
		dtos[i] = domain.ToCollectionUserDTO(u)
	}

	c.JSON(http.StatusOK, dtos)
}

// GetTeamAccess godoc
// @Summary Get collection team access
// @Description Get list of teams with access to collection
// @Tags collections
// @Produce json
// @Param id path int true "Collection ID"
// @Success 200 {array} domain.CollectionTeamDTO
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Router /collections/{id}/teams [get]
func (h *CollectionHandler) GetTeamAccess(c *gin.Context) {
	ctx := c.Request.Context()
	userID := GetCurrentUserID(c)

	collectionID, ok := GetUintParam(c, "id")
	if !ok {
		return
	}

	teams, err := h.service.GetTeamAccess(ctx, collectionID, userID)
	if err != nil {
		if errors.Is(err, repository.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get team access"})
		return
	}

	dtos := make([]*domain.CollectionTeamDTO, len(teams))
	for i, t := range teams {
		dtos[i] = domain.ToCollectionTeamDTO(t)
	}

	c.JSON(http.StatusOK, dtos)
}

