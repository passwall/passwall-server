package domain

import (
	"time"

	uuid "github.com/satori/go.uuid"
)

// Collection represents a shared folder at organization level
type Collection struct {
	ID        uint       `gorm:"primary_key" json:"id"`
	UUID      uuid.UUID  `gorm:"type:uuid;not null" json:"uuid"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty" gorm:"index"`

	OrganizationID uint `json:"organization_id" gorm:"not null;index;constraint:OnDelete:CASCADE"`

	// Collection details
	Name        string `json:"name" gorm:"type:varchar(255);not null"`
	Description string `json:"description,omitempty" gorm:"type:text"`

	// Access control
	IsPrivate bool `json:"is_private" gorm:"default:false"` // Only assigned users can access

	// External ID for LDAP/AD sync
	ExternalID *string `json:"external_id,omitempty" gorm:"type:varchar(255);index"`

	// Stats (runtime calculated, not stored in DB)
	ItemCount *int `json:"item_count,omitempty" gorm:"-"`
	UserCount *int `json:"user_count,omitempty" gorm:"-"`
	TeamCount *int `json:"team_count,omitempty" gorm:"-"`

	// Associations
	Organization *Organization      `json:"organization,omitempty" gorm:"foreignKey:OrganizationID"`
	UserAccess   []CollectionUser   `json:"user_access,omitempty" gorm:"foreignKey:CollectionID"`
	TeamAccess   []CollectionTeam   `json:"team_access,omitempty" gorm:"foreignKey:CollectionID"`
	Items        []OrganizationItem `json:"items,omitempty" gorm:"foreignKey:CollectionID"`
}

// TableName specifies the table name
func (Collection) TableName() string {
	return "collections"
}

// CollectionUser represents user permissions for a collection
type CollectionUser struct {
	ID        uint      `gorm:"primary_key" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	CollectionID       uint `json:"collection_id" gorm:"not null;index;constraint:OnDelete:CASCADE"`
	OrganizationUserID uint `json:"organization_user_id" gorm:"not null;index;constraint:OnDelete:CASCADE"`

	// Permissions
	CanRead      bool `json:"can_read" gorm:"default:true"`
	CanWrite     bool `json:"can_write" gorm:"default:false"`
	CanAdmin     bool `json:"can_admin" gorm:"default:false"`
	HidePasswords bool `json:"hide_passwords" gorm:"default:false"` // Can view metadata but not passwords

	// Associations
	Collection       *Collection       `json:"collection,omitempty" gorm:"foreignKey:CollectionID"`
	OrganizationUser *OrganizationUser `json:"organization_user,omitempty" gorm:"foreignKey:OrganizationUserID"`
}

// TableName specifies the table name
func (CollectionUser) TableName() string {
	return "collection_users"
}

// CollectionTeam represents team permissions for a collection
type CollectionTeam struct {
	ID        uint      `gorm:"primary_key" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	CollectionID uint `json:"collection_id" gorm:"not null;index;constraint:OnDelete:CASCADE"`
	TeamID       uint `json:"team_id" gorm:"not null;index;constraint:OnDelete:CASCADE"`

	// Permissions
	CanRead       bool `json:"can_read" gorm:"default:true"`
	CanWrite      bool `json:"can_write" gorm:"default:false"`
	CanAdmin      bool `json:"can_admin" gorm:"default:false"`
	HidePasswords bool `json:"hide_passwords" gorm:"default:false"`

	// Associations
	Collection *Collection `json:"collection,omitempty" gorm:"foreignKey:CollectionID"`
	Team       *Team       `json:"team,omitempty" gorm:"foreignKey:TeamID"`
}

// TableName specifies the table name
func (CollectionTeam) TableName() string {
	return "collection_teams"
}

// CollectionDTO for API responses
type CollectionDTO struct {
	ID             uint      `json:"id"`
	UUID           uuid.UUID `json:"uuid"`
	OrganizationID uint      `json:"organization_id"`
	Name           string    `json:"name"`
	Description    string    `json:"description,omitempty"`
	IsPrivate      bool      `json:"is_private"`
	ExternalID     *string   `json:"external_id,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`

	// Stats (optional)
	ItemCount *int `json:"item_count,omitempty"`
	UserCount *int `json:"user_count,omitempty"`
	TeamCount *int `json:"team_count,omitempty"`
}

// CollectionUserDTO for API responses
type CollectionUserDTO struct {
	ID                 uint      `json:"id"`
	CollectionID       uint      `json:"collection_id"`
	OrganizationUserID uint      `json:"organization_user_id"`
	UserID             uint      `json:"user_id"`
	UserEmail          string    `json:"user_email"`
	UserName           string    `json:"user_name"`
	CanRead            bool      `json:"can_read"`
	CanWrite           bool      `json:"can_write"`
	CanAdmin           bool      `json:"can_admin"`
	HidePasswords      bool      `json:"hide_passwords"`
	CreatedAt          time.Time `json:"created_at"`
}

// CollectionTeamDTO for API responses
type CollectionTeamDTO struct {
	ID            uint      `json:"id"`
	CollectionID  uint      `json:"collection_id"`
	TeamID        uint      `json:"team_id"`
	TeamName      string    `json:"team_name"`
	CanRead       bool      `json:"can_read"`
	CanWrite      bool      `json:"can_write"`
	CanAdmin      bool      `json:"can_admin"`
	HidePasswords bool      `json:"hide_passwords"`
	CreatedAt     time.Time `json:"created_at"`
}

// CreateCollectionRequest for API requests
type CreateCollectionRequest struct {
	Name        string  `json:"name" validate:"required,max=255"`
	Description string  `json:"description,omitempty" validate:"max=1000"`
	IsPrivate   bool    `json:"is_private"`
	ExternalID  *string `json:"external_id,omitempty"`
}

// UpdateCollectionRequest for API requests
type UpdateCollectionRequest struct {
	Name        *string `json:"name,omitempty" validate:"omitempty,max=255"`
	Description *string `json:"description,omitempty" validate:"omitempty,max=1000"`
	IsPrivate   *bool   `json:"is_private,omitempty"`
}

// GrantCollectionAccessRequest for granting access to users/teams
type GrantCollectionAccessRequest struct {
	CanRead       bool `json:"can_read"`
	CanWrite      bool `json:"can_write"`
	CanAdmin      bool `json:"can_admin"`
	HidePasswords bool `json:"hide_passwords"`
}

// ToCollectionDTO converts Collection to DTO
func ToCollectionDTO(c *Collection) *CollectionDTO {
	if c == nil {
		return nil
	}

	return &CollectionDTO{
		ID:             c.ID,
		UUID:           c.UUID,
		OrganizationID: c.OrganizationID,
		Name:           c.Name,
		Description:    c.Description,
		IsPrivate:      c.IsPrivate,
		ExternalID:     c.ExternalID,
		CreatedAt:      c.CreatedAt,
		UpdatedAt:      c.UpdatedAt,
		ItemCount:      c.ItemCount,
		UserCount:      c.UserCount,
		TeamCount:      c.TeamCount,
	}
}

// ToCollectionUserDTO converts CollectionUser to DTO
func ToCollectionUserDTO(cu *CollectionUser) *CollectionUserDTO {
	if cu == nil {
		return nil
	}

	dto := &CollectionUserDTO{
		ID:                 cu.ID,
		CollectionID:       cu.CollectionID,
		OrganizationUserID: cu.OrganizationUserID,
		CanRead:            cu.CanRead,
		CanWrite:           cu.CanWrite,
		CanAdmin:           cu.CanAdmin,
		HidePasswords:      cu.HidePasswords,
		CreatedAt:          cu.CreatedAt,
	}

	// Add user info if loaded
	if cu.OrganizationUser != nil && cu.OrganizationUser.User != nil {
		dto.UserID = cu.OrganizationUser.UserID
		dto.UserEmail = cu.OrganizationUser.User.Email
		dto.UserName = cu.OrganizationUser.User.Name
	}

	return dto
}

// ToCollectionTeamDTO converts CollectionTeam to DTO
func ToCollectionTeamDTO(ct *CollectionTeam) *CollectionTeamDTO {
	if ct == nil {
		return nil
	}

	dto := &CollectionTeamDTO{
		ID:            ct.ID,
		CollectionID:  ct.CollectionID,
		TeamID:        ct.TeamID,
		CanRead:       ct.CanRead,
		CanWrite:      ct.CanWrite,
		CanAdmin:      ct.CanAdmin,
		HidePasswords: ct.HidePasswords,
		CreatedAt:     ct.CreatedAt,
	}

	// Add team info if loaded
	if ct.Team != nil {
		dto.TeamName = ct.Team.Name
	}

	return dto
}

