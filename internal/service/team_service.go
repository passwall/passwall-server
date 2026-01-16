package service

import (
	"context"
	"fmt"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
)

type teamService struct {
	teamRepo     repository.TeamRepository
	teamUserRepo repository.TeamUserRepository
	orgUserRepo  repository.OrganizationUserRepository
	orgRepo      repository.OrganizationRepository
	logger       Logger
}

// NewTeamService creates a new team service
func NewTeamService(
	teamRepo repository.TeamRepository,
	teamUserRepo repository.TeamUserRepository,
	orgUserRepo repository.OrganizationUserRepository,
	orgRepo repository.OrganizationRepository,
	logger Logger,
) TeamService {
	return &teamService{
		teamRepo:     teamRepo,
		teamUserRepo: teamUserRepo,
		orgUserRepo:  orgUserRepo,
		orgRepo:      orgRepo,
		logger:       logger,
	}
}

func (s *teamService) Create(ctx context.Context, orgID uint, userID uint, req *domain.CreateTeamRequest) (*domain.Team, error) {
	// Check if user can manage teams (admin or manager)
	orgUser, err := s.orgUserRepo.GetByOrgAndUser(ctx, orgID, userID)
	if err != nil {
		return nil, repository.ErrForbidden
	}

	if !orgUser.CanManageCollections() {
		return nil, repository.ErrForbidden
	}

	// Check if team name already exists in organization
	existing, err := s.teamRepo.GetByName(ctx, orgID, req.Name)
	if err == nil && existing != nil {
		return nil, fmt.Errorf("team with name '%s' already exists", req.Name)
	}

	// Create team
	team := &domain.Team{
		OrganizationID:       orgID,
		Name:                 req.Name,
		Description:          req.Description,
		AccessAllCollections: req.AccessAllCollections,
		ExternalID:           req.ExternalID,
	}

	if err := s.teamRepo.Create(ctx, team); err != nil {
		s.logger.Error("failed to create team", "org_id", orgID, "name", req.Name, "error", err)
		return nil, fmt.Errorf("failed to create team: %w", err)
	}

	s.logger.Info("team created", "team_id", team.ID, "org_id", orgID, "name", team.Name, "created_by", userID)
	return team, nil
}

func (s *teamService) GetByID(ctx context.Context, id uint, userID uint) (*domain.Team, error) {
	team, err := s.teamRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("team not found: %w", err)
	}

	// Check if user is member of the organization
	if err := s.checkOrgMembership(ctx, team.OrganizationID, userID); err != nil {
		return nil, err
	}

	return team, nil
}

func (s *teamService) ListByOrganization(ctx context.Context, orgID uint, userID uint) ([]*domain.Team, error) {
	// Check if user is member of organization
	if err := s.checkOrgMembership(ctx, orgID, userID); err != nil {
		return nil, err
	}

	teams, err := s.teamRepo.ListByOrganization(ctx, orgID)
	if err != nil {
		s.logger.Error("failed to list teams", "org_id", orgID, "error", err)
		return nil, fmt.Errorf("failed to list teams: %w", err)
	}

	return teams, nil
}

func (s *teamService) Update(ctx context.Context, id uint, userID uint, req *domain.UpdateTeamRequest) (*domain.Team, error) {
	team, err := s.teamRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("team not found: %w", err)
	}

	// Check if user can manage teams
	if err := s.checkTeamManagePermission(ctx, team.OrganizationID, userID); err != nil {
		return nil, err
	}

	// Update fields
	if req.Name != nil {
		// Check if new name conflicts
		existing, err := s.teamRepo.GetByName(ctx, team.OrganizationID, *req.Name)
		if err == nil && existing != nil && existing.ID != id {
			return nil, fmt.Errorf("team with name '%s' already exists", *req.Name)
		}
		team.Name = *req.Name
	}
	if req.Description != nil {
		team.Description = *req.Description
	}
	if req.AccessAllCollections != nil {
		team.AccessAllCollections = *req.AccessAllCollections
	}

	if err := s.teamRepo.Update(ctx, team); err != nil {
		s.logger.Error("failed to update team", "team_id", id, "error", err)
		return nil, fmt.Errorf("failed to update team: %w", err)
	}

	s.logger.Info("team updated", "team_id", id, "org_id", team.OrganizationID, "updated_by", userID)
	return team, nil
}

func (s *teamService) Delete(ctx context.Context, id uint, userID uint) error {
	team, err := s.teamRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("team not found: %w", err)
	}

	if team.IsDefault {
		return fmt.Errorf("cannot delete default team")
	}

	// Check if user can manage teams
	if err := s.checkTeamManagePermission(ctx, team.OrganizationID, userID); err != nil {
		return err
	}

	if err := s.teamRepo.Delete(ctx, id); err != nil {
		s.logger.Error("failed to delete team", "team_id", id, "error", err)
		return fmt.Errorf("failed to delete team: %w", err)
	}

	s.logger.Info("team deleted", "team_id", id, "org_id", team.OrganizationID, "deleted_by", userID)
	return nil
}

func (s *teamService) AddMember(ctx context.Context, teamID uint, userID uint, req *domain.AddTeamUserRequest) error {
	team, err := s.teamRepo.GetByID(ctx, teamID)
	if err != nil {
		return fmt.Errorf("team not found: %w", err)
	}

	// Check if requesting user can manage team members
	if err := s.checkTeamMemberManagePermission(ctx, team.OrganizationID, teamID, userID); err != nil {
		return err
	}

	// Verify that the target user is a member of the organization
	targetOrgUser, err := s.orgUserRepo.GetByID(ctx, req.OrganizationUserID)
	if err != nil {
		return fmt.Errorf("organization user not found: %w", err)
	}

	if targetOrgUser.OrganizationID != team.OrganizationID {
		return fmt.Errorf("user is not a member of this organization")
	}

	// Check if already a member
	existing, err := s.teamUserRepo.GetByTeamAndOrgUser(ctx, teamID, req.OrganizationUserID)
	if err == nil && existing != nil {
		return fmt.Errorf("user is already a member of this team")
	}

	// Add to team
	teamUser := &domain.TeamUser{
		TeamID:             teamID,
		OrganizationUserID: req.OrganizationUserID,
		IsManager:          req.IsManager,
	}

	if err := s.teamUserRepo.Create(ctx, teamUser); err != nil {
		s.logger.Error("failed to add team member", "team_id", teamID, "org_user_id", req.OrganizationUserID, "error", err)
		return fmt.Errorf("failed to add team member: %w", err)
	}

	s.logger.Info("member added to team", "team_id", teamID, "org_user_id", req.OrganizationUserID, "is_manager", req.IsManager)
	return nil
}

func (s *teamService) GetMembers(ctx context.Context, teamID uint, userID uint) ([]*domain.TeamUser, error) {
	team, err := s.teamRepo.GetByID(ctx, teamID)
	if err != nil {
		return nil, fmt.Errorf("team not found: %w", err)
	}

	// Check if user is member of organization
	if err := s.checkOrgMembership(ctx, team.OrganizationID, userID); err != nil {
		return nil, err
	}

	members, err := s.teamUserRepo.ListByTeam(ctx, teamID)
	if err != nil {
		s.logger.Error("failed to get team members", "team_id", teamID, "error", err)
		return nil, fmt.Errorf("failed to get team members: %w", err)
	}

	return members, nil
}

func (s *teamService) UpdateMember(ctx context.Context, teamID uint, teamUserID uint, userID uint, req *domain.UpdateTeamUserRequest) error {
	team, err := s.teamRepo.GetByID(ctx, teamID)
	if err != nil {
		return fmt.Errorf("team not found: %w", err)
	}

	// Check if user can manage team members
	if err := s.checkTeamMemberManagePermission(ctx, team.OrganizationID, teamID, userID); err != nil {
		return err
	}

	teamUser, err := s.teamUserRepo.GetByID(ctx, teamUserID)
	if err != nil {
		return fmt.Errorf("team member not found: %w", err)
	}

	// Update
	teamUser.IsManager = req.IsManager

	if err := s.teamUserRepo.Update(ctx, teamUser); err != nil {
		s.logger.Error("failed to update team member", "team_user_id", teamUserID, "error", err)
		return fmt.Errorf("failed to update team member: %w", err)
	}

	s.logger.Info("team member updated", "team_id", teamID, "team_user_id", teamUserID, "is_manager", req.IsManager)
	return nil
}

func (s *teamService) RemoveMember(ctx context.Context, teamID uint, teamUserID uint, userID uint) error {
	team, err := s.teamRepo.GetByID(ctx, teamID)
	if err != nil {
		return fmt.Errorf("team not found: %w", err)
	}

	// Check if user can manage team members
	if err := s.checkTeamMemberManagePermission(ctx, team.OrganizationID, teamID, userID); err != nil {
		return err
	}

	if err := s.teamUserRepo.Delete(ctx, teamUserID); err != nil {
		s.logger.Error("failed to remove team member", "team_user_id", teamUserID, "error", err)
		return fmt.Errorf("failed to remove team member: %w", err)
	}

	s.logger.Info("team member removed", "team_id", teamID, "team_user_id", teamUserID)
	return nil
}

// Helper methods for permission checking

func (s *teamService) checkOrgMembership(ctx context.Context, orgID, userID uint) error {
	_, err := s.orgUserRepo.GetByOrgAndUser(ctx, orgID, userID)
	if err != nil {
		if err == repository.ErrNotFound {
			return repository.ErrForbidden
		}
		return err
	}
	return nil
}

func (s *teamService) checkTeamManagePermission(ctx context.Context, orgID, userID uint) error {
	orgUser, err := s.orgUserRepo.GetByOrgAndUser(ctx, orgID, userID)
	if err != nil {
		return repository.ErrForbidden
	}

	if !orgUser.CanManageCollections() {
		return repository.ErrForbidden
	}

	return nil
}

func (s *teamService) checkTeamMemberManagePermission(ctx context.Context, orgID, teamID, userID uint) error {
	// Organization admins can always manage
	orgUser, err := s.orgUserRepo.GetByOrgAndUser(ctx, orgID, userID)
	if err != nil {
		return repository.ErrForbidden
	}

	if orgUser.IsAdmin() {
		return nil
	}

	// Check if user is a team manager
	teamMembers, err := s.teamUserRepo.ListByTeam(ctx, teamID)
	if err != nil {
		return err
	}

	for _, tm := range teamMembers {
		if tm.OrganizationUserID == orgUser.ID && tm.IsManager {
			return nil
		}
	}

	return repository.ErrForbidden
}
