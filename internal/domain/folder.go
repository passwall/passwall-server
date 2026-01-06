package domain

import (
	"time"

	uuid "github.com/satori/go.uuid"
)

// Folder represents a folder/collection for organizing vault items
type Folder struct {
	ID        uint      `gorm:"primary_key" json:"id"`
	UUID      uuid.UUID `gorm:"type:uuid;type:varchar(100);" json:"uuid"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	UserID uint   `json:"user_id" gorm:"not null;index"`
	Name   string `json:"name" gorm:"type:varchar(255);not null"`

	// Composite unique index on user_id + name
	// Each user can have a folder name only once
}

func (Folder) TableName() string {
	return "folders"
}

// FolderDTO for API responses
type FolderDTO struct {
	ID        uint      `json:"id"`
	UUID      uuid.UUID `json:"uuid"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

func ToFolderDTO(f *Folder) *FolderDTO {
	if f == nil {
		return nil
	}

	return &FolderDTO{
		ID:        f.ID,
		UUID:      f.UUID,
		Name:      f.Name,
		CreatedAt: f.CreatedAt,
	}
}

func ToFolderDTOs(folders []*Folder) []*FolderDTO {
	dtos := make([]*FolderDTO, len(folders))
	for i, f := range folders {
		dtos[i] = ToFolderDTO(f)
	}
	return dtos
}

// CreateFolderRequest for API requests
type CreateFolderRequest struct {
	Name string `json:"name" validate:"required,max=255"`
}

// UpdateFolderRequest for API requests
type UpdateFolderRequest struct {
	Name string `json:"name" validate:"required,max=255"`
}
