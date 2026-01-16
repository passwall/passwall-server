package http

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	"github.com/passwall/passwall-server/internal/service"
)

type TeamHandler struct {
	service service.TeamService
}

func NewTeamHandler(service service.TeamService) *TeamHandler {
	return &TeamHandler{service: service}
}

// Create godoc
// @Summary Create team
// @Description Create a new team in an organization
// @Tags teams
// @Accept json
// @Produce json
// @Param orgId path int true "Organization ID"
// @Param request body domain.CreateTeamRequest true "Team details"
// @Success 201 {object} domain.TeamDTO
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Router /organizations/{orgId}/teams [post]
func (h *TeamHandler) Create(c *gin.Context) {
	ctx := c.Request.Context()
	userID := GetCurrentUserID(c)

	orgID, ok := GetUintParam(c, "id")
	if !ok {
		return
	}

	var req domain.CreateTeamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "details": err.Error()})
		return
	}

	team, err := h.service.Create(ctx, orgID, userID, &req)
	if err != nil {
		if errors.Is(err, repository.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to create team", "details": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, domain.ToTeamDTO(team))
}

// List godoc
// @Summary List teams
// @Description List teams in an organization
// @Tags teams
// @Produce json
// @Param orgId path int true "Organization ID"
// @Success 200 {array} domain.TeamDTO
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Router /organizations/{orgId}/teams [get]
func (h *TeamHandler) List(c *gin.Context) {
	ctx := c.Request.Context()
	userID := GetCurrentUserID(c)

	orgID, ok := GetUintParam(c, "id")
	if !ok {
		return
	}

	teams, err := h.service.ListByOrganization(ctx, orgID, userID)
	if err != nil {
		if errors.Is(err, repository.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list teams"})
		return
	}

	dtos := make([]*domain.TeamDTO, len(teams))
	for i, team := range teams {
		dtos[i] = domain.ToTeamDTO(team)
	}

	c.JSON(http.StatusOK, dtos)
}

// GetByID godoc
// @Summary Get team
// @Description Get team by ID
// @Tags teams
// @Produce json
// @Param id path int true "Team ID"
// @Success 200 {object} domain.TeamDTO
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /teams/{id} [get]
func (h *TeamHandler) GetByID(c *gin.Context) {
	ctx := c.Request.Context()
	userID := GetCurrentUserID(c)

	id, ok := GetUintParam(c, "id")
	if !ok {
		return
	}

	team, err := h.service.GetByID(ctx, id, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "team not found"})
			return
		}
		if errors.Is(err, repository.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get team"})
		return
	}

	c.JSON(http.StatusOK, domain.ToTeamDTO(team))
}

// Update godoc
// @Summary Update team
// @Description Update team details
// @Tags teams
// @Accept json
// @Produce json
// @Param id path int true "Team ID"
// @Param request body domain.UpdateTeamRequest true "Team details"
// @Success 200 {object} domain.TeamDTO
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Router /teams/{id} [put]
func (h *TeamHandler) Update(c *gin.Context) {
	ctx := c.Request.Context()
	userID := GetCurrentUserID(c)

	id, ok := GetUintParam(c, "id")
	if !ok {
		return
	}

	var req domain.UpdateTeamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "details": err.Error()})
		return
	}

	team, err := h.service.Update(ctx, id, userID, &req)
	if err != nil {
		if errors.Is(err, repository.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to update team", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, domain.ToTeamDTO(team))
}

// Delete godoc
// @Summary Delete team
// @Description Delete a team
// @Tags teams
// @Param id path int true "Team ID"
// @Success 204
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Router /teams/{id} [delete]
func (h *TeamHandler) Delete(c *gin.Context) {
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete team"})
		return
	}

	c.Status(http.StatusNoContent)
}

// AddMember godoc
// @Summary Add member to team
// @Description Add a user to the team
// @Tags teams
// @Accept json
// @Produce json
// @Param id path int true "Team ID"
// @Param request body domain.AddTeamUserRequest true "Member details"
// @Success 201 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Router /teams/{id}/members [post]
func (h *TeamHandler) AddMember(c *gin.Context) {
	ctx := c.Request.Context()
	userID := GetCurrentUserID(c)

	teamID, ok := GetUintParam(c, "id")
	if !ok {
		return
	}

	var req domain.AddTeamUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "details": err.Error()})
		return
	}

	err := h.service.AddMember(ctx, teamID, userID, &req)
	if err != nil {
		if errors.Is(err, repository.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to add member", "details": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "member added successfully"})
}

// GetMembers godoc
// @Summary Get team members
// @Description Get list of team members
// @Tags teams
// @Produce json
// @Param id path int true "Team ID"
// @Success 200 {array} domain.TeamUserDTO
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Router /teams/{id}/members [get]
func (h *TeamHandler) GetMembers(c *gin.Context) {
	ctx := c.Request.Context()
	userID := GetCurrentUserID(c)

	teamID, ok := GetUintParam(c, "id")
	if !ok {
		return
	}

	members, err := h.service.GetMembers(ctx, teamID, userID)
	if err != nil {
		if errors.Is(err, repository.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get members"})
		return
	}

	dtos := make([]*domain.TeamUserDTO, len(members))
	for i, m := range members {
		dtos[i] = domain.ToTeamUserDTO(m)
	}

	c.JSON(http.StatusOK, dtos)
}

// UpdateMember godoc
// @Summary Update team member
// @Description Update team member role
// @Tags teams
// @Accept json
// @Produce json
// @Param id path int true "Team ID"
// @Param memberId path int true "Team Member ID"
// @Param request body domain.UpdateTeamUserRequest true "Member details"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Router /teams/{id}/members/{memberId} [put]
func (h *TeamHandler) UpdateMember(c *gin.Context) {
	ctx := c.Request.Context()
	userID := GetCurrentUserID(c)

	teamID, ok := GetUintParam(c, "id")
	if !ok {
		return
	}

	memberID, ok := GetUintParam(c, "memberId")
	if !ok {
		return
	}

	var req domain.UpdateTeamUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "details": err.Error()})
		return
	}

	err := h.service.UpdateMember(ctx, teamID, memberID, userID, &req)
	if err != nil {
		if errors.Is(err, repository.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to update member", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "member updated successfully"})
}

// RemoveMember godoc
// @Summary Remove member from team
// @Description Remove a member from the team
// @Tags teams
// @Param id path int true "Team ID"
// @Param memberId path int true "Team Member ID"
// @Success 204
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Router /teams/{id}/members/{memberId} [delete]
func (h *TeamHandler) RemoveMember(c *gin.Context) {
	ctx := c.Request.Context()
	userID := GetCurrentUserID(c)

	teamID, ok := GetUintParam(c, "id")
	if !ok {
		return
	}

	memberID, ok := GetUintParam(c, "memberId")
	if !ok {
		return
	}

	err := h.service.RemoveMember(ctx, teamID, memberID, userID)
	if err != nil {
		if errors.Is(err, repository.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to remove member"})
		return
	}

	c.Status(http.StatusNoContent)
}
