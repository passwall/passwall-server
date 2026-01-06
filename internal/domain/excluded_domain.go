package domain

import (
	"time"

	uuid "github.com/satori/go.uuid"
)

// ExcludedDomain represents a domain where Passwall is disabled for a specific user
type ExcludedDomain struct {
	ID        uint      `gorm:"primary_key" json:"id"`
	UUID      uuid.UUID `gorm:"type:uuid;type:varchar(100);" json:"uuid"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	UserID uint   `json:"user_id" gorm:"not null;index"`
	Domain string `json:"domain" gorm:"type:varchar(255);not null"`

	// Composite unique index on user_id + domain
	// Each user can exclude a domain only once
}

func (ExcludedDomain) TableName() string {
	return "excluded_domains"
}

// ExcludedDomainDTO for API responses
type ExcludedDomainDTO struct {
	ID        uint      `json:"id"`
	UUID      uuid.UUID `json:"uuid"`
	Domain    string    `json:"domain"`
	CreatedAt time.Time `json:"created_at"`
}

func ToExcludedDomainDTO(ed *ExcludedDomain) *ExcludedDomainDTO {
	if ed == nil {
		return nil
	}

	return &ExcludedDomainDTO{
		ID:        ed.ID,
		UUID:      ed.UUID,
		Domain:    ed.Domain,
		CreatedAt: ed.CreatedAt,
	}
}

func ToExcludedDomainDTOs(eds []*ExcludedDomain) []*ExcludedDomainDTO {
	dtos := make([]*ExcludedDomainDTO, len(eds))
	for i, ed := range eds {
		dtos[i] = ToExcludedDomainDTO(ed)
	}
	return dtos
}

// CreateExcludedDomainRequest for API requests
type CreateExcludedDomainRequest struct {
	Domain string `json:"domain" validate:"required"`
}
