package repository

import (
	"context"

	"github.com/passwall/passwall-server/internal/domain"
)

// OrganizationFolderRepository defines organization folder data access methods
type OrganizationFolderRepository interface {
	Create(ctx context.Context, folder *domain.OrganizationFolder) error
	GetByID(ctx context.Context, id uint) (*domain.OrganizationFolder, error)
	GetByOrganization(ctx context.Context, orgID uint) ([]*domain.OrganizationFolder, error)
	GetByOrganizationAndName(ctx context.Context, orgID uint, name string) (*domain.OrganizationFolder, error)
	Update(ctx context.Context, folder *domain.OrganizationFolder) error
	Delete(ctx context.Context, id uint) error
}
