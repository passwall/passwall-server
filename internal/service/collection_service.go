package service

import (
	"context"
	"fmt"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
)

type collectionService struct {
	collectionRepo     repository.CollectionRepository
	collectionUserRepo repository.CollectionUserRepository
	collectionTeamRepo repository.CollectionTeamRepository
	orgUserRepo        repository.OrganizationUserRepository
	teamRepo           repository.TeamRepository
	orgRepo            repository.OrganizationRepository
	logger             Logger
}

// NewCollectionService creates a new collection service
func NewCollectionService(
	collectionRepo repository.CollectionRepository,
	collectionUserRepo repository.CollectionUserRepository,
	collectionTeamRepo repository.CollectionTeamRepository,
	orgUserRepo repository.OrganizationUserRepository,
	teamRepo repository.TeamRepository,
	orgRepo repository.OrganizationRepository,
	logger Logger,
) CollectionService {
	return &collectionService{
		collectionRepo:     collectionRepo,
		collectionUserRepo: collectionUserRepo,
		collectionTeamRepo: collectionTeamRepo,
		orgUserRepo:        orgUserRepo,
		teamRepo:           teamRepo,
		orgRepo:            orgRepo,
		logger:             logger,
	}
}

func (s *collectionService) Create(ctx context.Context, orgID uint, userID uint, req *domain.CreateCollectionRequest) (*domain.Collection, error) {
	// Check if user can manage collections
	orgUser, err := s.orgUserRepo.GetByOrgAndUser(ctx, orgID, userID)
	if err != nil {
		return nil, repository.ErrForbidden
	}

	if !orgUser.CanManageCollections() {
		return nil, repository.ErrForbidden
	}

	// Check organization collection limit
	org, err := s.orgRepo.GetByID(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("organization not found: %w", err)
	}

	collectionCount, err := s.orgRepo.GetCollectionCount(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to get collection count: %w", err)
	}

	if collectionCount >= org.MaxCollections {
		return nil, fmt.Errorf("organization has reached max collections limit (%d)", org.MaxCollections)
	}

	// Check if collection name already exists
	existing, err := s.collectionRepo.GetByName(ctx, orgID, req.Name)
	if err == nil && existing != nil {
		return nil, fmt.Errorf("collection with name '%s' already exists", req.Name)
	}

	// Create collection
	collection := &domain.Collection{
		OrganizationID: orgID,
		Name:           req.Name,
		Description:    req.Description,
		IsPrivate:      req.IsPrivate,
		ExternalID:     req.ExternalID,
	}

	if err := s.collectionRepo.Create(ctx, collection); err != nil {
		s.logger.Error("failed to create collection", "org_id", orgID, "name", req.Name, "error", err)
		return nil, fmt.Errorf("failed to create collection: %w", err)
	}

	s.logger.Info("collection created", "collection_id", collection.ID, "org_id", orgID, "name", collection.Name, "created_by", userID)
	return collection, nil
}

func (s *collectionService) GetByID(ctx context.Context, id uint, userID uint) (*domain.Collection, error) {
	collection, err := s.collectionRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("collection not found: %w", err)
	}

	// Check if user has access to this collection
	hasAccess, err := s.checkCollectionAccess(ctx, collection.OrganizationID, id, userID)
	if err != nil {
		return nil, err
	}

	if !hasAccess {
		return nil, repository.ErrForbidden
	}

	return collection, nil
}

func (s *collectionService) ListByOrganization(ctx context.Context, orgID uint, userID uint) ([]*domain.Collection, error) {
	// Check if user is org admin (can see all collections)
	orgUser, err := s.orgUserRepo.GetByOrgAndUser(ctx, orgID, userID)
	if err != nil {
		return nil, repository.ErrForbidden
	}

	// Admins or users with access_all can see all collections
	if orgUser.IsAdmin() || orgUser.AccessAll {
		collections, err := s.collectionRepo.ListByOrganization(ctx, orgID)
		if err != nil {
			s.logger.Error("failed to list collections", "org_id", orgID, "error", err)
			return nil, fmt.Errorf("failed to list collections: %w", err)
		}
		return collections, nil
	}

	// Regular users only see collections they have access to
	return s.ListForUser(ctx, orgID, userID)
}

func (s *collectionService) ListForUser(ctx context.Context, orgID uint, userID uint) ([]*domain.Collection, error) {
	// Check if user is member of organization
	_, err := s.orgUserRepo.GetByOrgAndUser(ctx, orgID, userID)
	if err != nil {
		return nil, repository.ErrForbidden
	}

	collections, err := s.collectionRepo.ListForUser(ctx, orgID, userID)
	if err != nil {
		s.logger.Error("failed to list user collections", "org_id", orgID, "user_id", userID, "error", err)
		return nil, fmt.Errorf("failed to list collections: %w", err)
	}

	return collections, nil
}

func (s *collectionService) Update(ctx context.Context, id uint, userID uint, req *domain.UpdateCollectionRequest) (*domain.Collection, error) {
	collection, err := s.collectionRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("collection not found: %w", err)
	}

	// Check if user can manage this collection
	canManage, err := s.checkCollectionManagePermission(ctx, collection.OrganizationID, id, userID)
	if err != nil {
		return nil, err
	}

	if !canManage {
		return nil, repository.ErrForbidden
	}

	// Update fields
	if req.Name != nil {
		// Check name conflict
		existing, err := s.collectionRepo.GetByName(ctx, collection.OrganizationID, *req.Name)
		if err == nil && existing != nil && existing.ID != id {
			return nil, fmt.Errorf("collection with name '%s' already exists", *req.Name)
		}
		collection.Name = *req.Name
	}
	if req.Description != nil {
		collection.Description = *req.Description
	}
	if req.IsPrivate != nil {
		collection.IsPrivate = *req.IsPrivate
	}

	if err := s.collectionRepo.Update(ctx, collection); err != nil {
		s.logger.Error("failed to update collection", "collection_id", id, "error", err)
		return nil, fmt.Errorf("failed to update collection: %w", err)
	}

	s.logger.Info("collection updated", "collection_id", id, "org_id", collection.OrganizationID, "updated_by", userID)
	return collection, nil
}

func (s *collectionService) Delete(ctx context.Context, id uint, userID uint) error {
	collection, err := s.collectionRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("collection not found: %w", err)
	}

	// Only org admins can delete collections
	orgUser, err := s.orgUserRepo.GetByOrgAndUser(ctx, collection.OrganizationID, userID)
	if err != nil {
		return repository.ErrForbidden
	}

	if !orgUser.IsAdmin() {
		return repository.ErrForbidden
	}

	// Use soft delete
	if err := s.collectionRepo.SoftDelete(ctx, id); err != nil {
		s.logger.Error("failed to delete collection", "collection_id", id, "error", err)
		return fmt.Errorf("failed to delete collection: %w", err)
	}

	s.logger.Info("collection deleted", "collection_id", id, "org_id", collection.OrganizationID, "deleted_by", userID)
	return nil
}

func (s *collectionService) GrantUserAccess(ctx context.Context, collectionID uint, orgUserID uint, requestingUserID uint, req *domain.GrantCollectionAccessRequest) error {
	collection, err := s.collectionRepo.GetByID(ctx, collectionID)
	if err != nil {
		return fmt.Errorf("collection not found: %w", err)
	}

	// Check if requesting user can manage this collection
	canManage, err := s.checkCollectionManagePermission(ctx, collection.OrganizationID, collectionID, requestingUserID)
	if err != nil {
		return err
	}

	if !canManage {
		return repository.ErrForbidden
	}

	// Verify target user is in the organization
	targetOrgUser, err := s.orgUserRepo.GetByID(ctx, orgUserID)
	if err != nil {
		return fmt.Errorf("organization user not found: %w", err)
	}

	if targetOrgUser.OrganizationID != collection.OrganizationID {
		return fmt.Errorf("user is not a member of this organization")
	}

	// Check if access already exists
	existing, err := s.collectionUserRepo.GetByCollectionAndOrgUser(ctx, collectionID, orgUserID)
	if err == nil && existing != nil {
		// Update existing access
		existing.CanRead = req.CanRead
		existing.CanWrite = req.CanWrite
		existing.CanAdmin = req.CanAdmin
		existing.HidePasswords = req.HidePasswords

		if err := s.collectionUserRepo.Update(ctx, existing); err != nil {
			s.logger.Error("failed to update collection user access", "collection_id", collectionID, "org_user_id", orgUserID, "error", err)
			return fmt.Errorf("failed to update access: %w", err)
		}

		s.logger.Info("collection user access updated", "collection_id", collectionID, "org_user_id", orgUserID)
		return nil
	}

	// Create new access
	collectionUser := &domain.CollectionUser{
		CollectionID:       collectionID,
		OrganizationUserID: orgUserID,
		CanRead:            req.CanRead,
		CanWrite:           req.CanWrite,
		CanAdmin:           req.CanAdmin,
		HidePasswords:      req.HidePasswords,
	}

	if err := s.collectionUserRepo.Create(ctx, collectionUser); err != nil {
		s.logger.Error("failed to grant collection user access", "collection_id", collectionID, "org_user_id", orgUserID, "error", err)
		return fmt.Errorf("failed to grant access: %w", err)
	}

	s.logger.Info("collection user access granted", "collection_id", collectionID, "org_user_id", orgUserID)
	return nil
}

func (s *collectionService) GrantTeamAccess(ctx context.Context, collectionID uint, teamID uint, requestingUserID uint, req *domain.GrantCollectionAccessRequest) error {
	collection, err := s.collectionRepo.GetByID(ctx, collectionID)
	if err != nil {
		return fmt.Errorf("collection not found: %w", err)
	}

	// Check if requesting user can manage this collection
	canManage, err := s.checkCollectionManagePermission(ctx, collection.OrganizationID, collectionID, requestingUserID)
	if err != nil {
		return err
	}

	if !canManage {
		return repository.ErrForbidden
	}

	// Verify team is in the same organization
	team, err := s.teamRepo.GetByID(ctx, teamID)
	if err != nil {
		return fmt.Errorf("team not found: %w", err)
	}

	if team.OrganizationID != collection.OrganizationID {
		return fmt.Errorf("team is not in the same organization")
	}

	// Check if access already exists
	existing, err := s.collectionTeamRepo.GetByCollectionAndTeam(ctx, collectionID, teamID)
	if err == nil && existing != nil {
		// Update existing access
		existing.CanRead = req.CanRead
		existing.CanWrite = req.CanWrite
		existing.CanAdmin = req.CanAdmin
		existing.HidePasswords = req.HidePasswords

		if err := s.collectionTeamRepo.Update(ctx, existing); err != nil {
			s.logger.Error("failed to update collection team access", "collection_id", collectionID, "team_id", teamID, "error", err)
			return fmt.Errorf("failed to update access: %w", err)
		}

		s.logger.Info("collection team access updated", "collection_id", collectionID, "team_id", teamID)
		return nil
	}

	// Create new access
	collectionTeam := &domain.CollectionTeam{
		CollectionID:  collectionID,
		TeamID:        teamID,
		CanRead:       req.CanRead,
		CanWrite:      req.CanWrite,
		CanAdmin:      req.CanAdmin,
		HidePasswords: req.HidePasswords,
	}

	if err := s.collectionTeamRepo.Create(ctx, collectionTeam); err != nil {
		s.logger.Error("failed to grant collection team access", "collection_id", collectionID, "team_id", teamID, "error", err)
		return fmt.Errorf("failed to grant access: %w", err)
	}

	s.logger.Info("collection team access granted", "collection_id", collectionID, "team_id", teamID)
	return nil
}

func (s *collectionService) RevokeUserAccess(ctx context.Context, collectionID uint, orgUserID uint, requestingUserID uint) error {
	collection, err := s.collectionRepo.GetByID(ctx, collectionID)
	if err != nil {
		return fmt.Errorf("collection not found: %w", err)
	}

	// Check if requesting user can manage this collection
	canManage, err := s.checkCollectionManagePermission(ctx, collection.OrganizationID, collectionID, requestingUserID)
	if err != nil {
		return err
	}

	if !canManage {
		return repository.ErrForbidden
	}

	if err := s.collectionUserRepo.DeleteByCollectionAndOrgUser(ctx, collectionID, orgUserID); err != nil {
		s.logger.Error("failed to revoke collection user access", "collection_id", collectionID, "org_user_id", orgUserID, "error", err)
		return fmt.Errorf("failed to revoke access: %w", err)
	}

	s.logger.Info("collection user access revoked", "collection_id", collectionID, "org_user_id", orgUserID)
	return nil
}

func (s *collectionService) RevokeTeamAccess(ctx context.Context, collectionID uint, teamID uint, requestingUserID uint) error {
	collection, err := s.collectionRepo.GetByID(ctx, collectionID)
	if err != nil {
		return fmt.Errorf("collection not found: %w", err)
	}

	// Check if requesting user can manage this collection
	canManage, err := s.checkCollectionManagePermission(ctx, collection.OrganizationID, collectionID, requestingUserID)
	if err != nil {
		return err
	}

	if !canManage {
		return repository.ErrForbidden
	}

	if err := s.collectionTeamRepo.DeleteByCollectionAndTeam(ctx, collectionID, teamID); err != nil {
		s.logger.Error("failed to revoke collection team access", "collection_id", collectionID, "team_id", teamID, "error", err)
		return fmt.Errorf("failed to revoke access: %w", err)
	}

	s.logger.Info("collection team access revoked", "collection_id", collectionID, "team_id", teamID)
	return nil
}

func (s *collectionService) GetUserAccess(ctx context.Context, collectionID uint, requestingUserID uint) ([]*domain.CollectionUser, error) {
	collection, err := s.collectionRepo.GetByID(ctx, collectionID)
	if err != nil {
		return nil, fmt.Errorf("collection not found: %w", err)
	}

	// Check if requesting user can view access (must have access to collection)
	hasAccess, err := s.checkCollectionAccess(ctx, collection.OrganizationID, collectionID, requestingUserID)
	if err != nil {
		return nil, err
	}

	if !hasAccess {
		return nil, repository.ErrForbidden
	}

	users, err := s.collectionUserRepo.ListByCollection(ctx, collectionID)
	if err != nil {
		s.logger.Error("failed to get collection users", "collection_id", collectionID, "error", err)
		return nil, fmt.Errorf("failed to get collection users: %w", err)
	}

	return users, nil
}

func (s *collectionService) GetTeamAccess(ctx context.Context, collectionID uint, requestingUserID uint) ([]*domain.CollectionTeam, error) {
	collection, err := s.collectionRepo.GetByID(ctx, collectionID)
	if err != nil {
		return nil, fmt.Errorf("collection not found: %w", err)
	}

	// Check if requesting user can view access
	hasAccess, err := s.checkCollectionAccess(ctx, collection.OrganizationID, collectionID, requestingUserID)
	if err != nil {
		return nil, err
	}

	if !hasAccess {
		return nil, repository.ErrForbidden
	}

	teams, err := s.collectionTeamRepo.ListByCollection(ctx, collectionID)
	if err != nil {
		s.logger.Error("failed to get collection teams", "collection_id", collectionID, "error", err)
		return nil, fmt.Errorf("failed to get collection teams: %w", err)
	}

	return teams, nil
}

// Helper methods for permission checking

func (s *collectionService) checkCollectionAccess(ctx context.Context, orgID, collectionID, userID uint) (bool, error) {
	orgUser, err := s.orgUserRepo.GetByOrgAndUser(ctx, orgID, userID)
	if err != nil {
		return false, repository.ErrForbidden
	}

	// Admins and users with access_all can access all collections
	if orgUser.IsAdmin() || orgUser.AccessAll {
		return true, nil
	}

	// Check direct user access
	collectionUser, err := s.collectionUserRepo.GetByCollectionAndOrgUser(ctx, collectionID, orgUser.ID)
	if err == nil && collectionUser != nil && collectionUser.CanRead {
		return true, nil
	}

	// Check team access
	teamUsers, err := s.orgUserRepo.ListByUser(ctx, userID)
	if err != nil {
		return false, err
	}

	for _, tu := range teamUsers {
		if tu.OrganizationID == orgID {
			// Get teams for this org user
			// This is simplified - in production you'd want to optimize this query
			collections, err := s.collectionRepo.ListForUser(ctx, orgID, userID)
			if err == nil {
				for _, c := range collections {
					if c.ID == collectionID {
						return true, nil
					}
				}
			}
		}
	}

	return false, nil
}

func (s *collectionService) checkCollectionManagePermission(ctx context.Context, orgID, collectionID, userID uint) (bool, error) {
	orgUser, err := s.orgUserRepo.GetByOrgAndUser(ctx, orgID, userID)
	if err != nil {
		return false, repository.ErrForbidden
	}

	// Org admins can manage all collections
	if orgUser.IsAdmin() {
		return true, nil
	}

	// Check if user has admin permission on this specific collection
	collectionUser, err := s.collectionUserRepo.GetByCollectionAndOrgUser(ctx, collectionID, orgUser.ID)
	if err == nil && collectionUser != nil && collectionUser.CanAdmin {
		return true, nil
	}

	return false, nil
}

