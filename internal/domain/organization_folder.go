package domain

import (
	"time"

	uuid "github.com/satori/go.uuid"
)

// OrganizationFolder represents a folder for organization vault items
type OrganizationFolder struct {
	ID        uint      `gorm:"primary_key" json:"id"`
	UUID      uuid.UUID `gorm:"type:uuid;type:varchar(100);" json:"uuid"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	OrganizationID  uint   `json:"organization_id" gorm:"not null;index;constraint:OnDelete:CASCADE"`
	CreatedByUserID uint   `json:"created_by_user_id" gorm:"not null;index"`
	Name            string `json:"name" gorm:"type:varchar(255);not null"`
}

func (OrganizationFolder) TableName() string {
	return "organization_folders"
}

type OrganizationFolderDTO struct {
	ID        uint      `json:"id"`
	UUID      uuid.UUID `json:"uuid"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

func ToOrganizationFolderDTO(f *OrganizationFolder) *OrganizationFolderDTO {
	if f == nil {
		return nil
	}

	return &OrganizationFolderDTO{
		ID:        f.ID,
		UUID:      f.UUID,
		Name:      f.Name,
		CreatedAt: f.CreatedAt,
	}
}

func ToOrganizationFolderDTOs(folders []*OrganizationFolder) []*OrganizationFolderDTO {
	dtos := make([]*OrganizationFolderDTO, len(folders))
	for i, f := range folders {
		dtos[i] = ToOrganizationFolderDTO(f)
	}
	return dtos
}

type CreateOrganizationFolderRequest struct {
	Name string `json:"name" validate:"required,max=255"`
}

type UpdateOrganizationFolderRequest struct {
	Name string `json:"name" validate:"required,max=255"`
}
