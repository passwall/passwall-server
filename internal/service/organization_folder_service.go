package service

import (
	"context"
	"fmt"

	"github.com/passwall/passwall-server/internal/domain"
	"github.com/passwall/passwall-server/internal/repository"
	uuid "github.com/satori/go.uuid"
)

type organizationFolderService struct {
	folderRepo  repository.OrganizationFolderRepository
	itemRepo    repository.OrganizationItemRepository
	orgUserRepo repository.OrganizationUserRepository
	logger      Logger
}

func NewOrganizationFolderService(
	folderRepo repository.OrganizationFolderRepository,
	itemRepo repository.OrganizationItemRepository,
	orgUserRepo repository.OrganizationUserRepository,
	logger Logger,
) OrganizationFolderService {
	return &organizationFolderService{
		folderRepo:  folderRepo,
		itemRepo:    itemRepo,
		orgUserRepo: orgUserRepo,
		logger:      logger,
	}
}

func (s *organizationFolderService) ListByOrganization(ctx context.Context, orgID, userID uint) ([]*domain.OrganizationFolder, error) {
	_, err := s.orgUserRepo.GetByOrgAndUser(ctx, orgID, userID)
	if err != nil {
		return nil, repository.ErrForbidden
	}

	return s.folderRepo.GetByOrganization(ctx, orgID)
}

func (s *organizationFolderService) Create(ctx context.Context, orgID, userID uint, req *domain.CreateOrganizationFolderRequest) (*domain.OrganizationFolder, error) {
	orgUser, err := s.orgUserRepo.GetByOrgAndUser(ctx, orgID, userID)
	if err != nil {
		return nil, repository.ErrForbidden
	}

	if !orgUser.CanManageCollections() {
		return nil, repository.ErrForbidden
	}

	if _, err := s.folderRepo.GetByOrganizationAndName(ctx, orgID, req.Name); err == nil {
		return nil, fmt.Errorf("folder name already exists")
	}

	folder := &domain.OrganizationFolder{
		UUID:            uuid.NewV4(),
		OrganizationID:  orgID,
		CreatedByUserID: userID,
		Name:            req.Name,
	}

	if err := s.folderRepo.Create(ctx, folder); err != nil {
		s.logger.Error("failed to create organization folder", "org_id", orgID, "error", err)
		return nil, fmt.Errorf("failed to create folder: %w", err)
	}

	return folder, nil
}

func (s *organizationFolderService) Update(ctx context.Context, orgID, userID, id uint, req *domain.UpdateOrganizationFolderRequest) (*domain.OrganizationFolder, error) {
	orgUser, err := s.orgUserRepo.GetByOrgAndUser(ctx, orgID, userID)
	if err != nil {
		return nil, repository.ErrForbidden
	}

	if !orgUser.CanManageCollections() {
		return nil, repository.ErrForbidden
	}

	folder, err := s.folderRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if folder.OrganizationID != orgID {
		return nil, repository.ErrForbidden
	}

	folder.Name = req.Name
	if err := s.folderRepo.Update(ctx, folder); err != nil {
		return nil, err
	}

	return folder, nil
}

func (s *organizationFolderService) Delete(ctx context.Context, orgID, userID, id uint) error {
	orgUser, err := s.orgUserRepo.GetByOrgAndUser(ctx, orgID, userID)
	if err != nil {
		return repository.ErrForbidden
	}

	if !orgUser.CanManageCollections() {
		return repository.ErrForbidden
	}

	folder, err := s.folderRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if folder.OrganizationID != orgID {
		return repository.ErrForbidden
	}

	// Prevent deleting folders with items
	items, _, err := s.itemRepo.ListByOrganization(ctx, repository.OrganizationItemFilter{
		OrganizationID: orgID,
		FolderID:       &folder.ID,
		PerPage:        1,
		Page:           1,
	})
	if err != nil {
		return err
	}
	if len(items) > 0 {
		return fmt.Errorf("cannot delete folder: contains items")
	}

	return s.folderRepo.Delete(ctx, id)
}
