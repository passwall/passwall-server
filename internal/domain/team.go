package domain

import (
	"time"

	uuid "github.com/satori/go.uuid"
)

// Team represents a group of users within an organization
type Team struct {
	ID        uint      `gorm:"primary_key" json:"id"`
	UUID      uuid.UUID `gorm:"type:uuid;not null" json:"uuid"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	OrganizationID uint `json:"organization_id" gorm:"not null;index;constraint:OnDelete:CASCADE"`

	// Team details
	Name        string `json:"name" gorm:"type:varchar(255);not null"`
	Description string `json:"description,omitempty" gorm:"type:text"`

	// System defaults
	IsDefault bool `json:"is_default" gorm:"not null;default:false"`

	// Access control
	AccessAllCollections bool `json:"access_all_collections" gorm:"default:false"`

	// External ID for LDAP/AD sync
	ExternalID *string `json:"external_id,omitempty" gorm:"type:varchar(255);index"`

	// Associations
	Organization *Organization `json:"organization,omitempty" gorm:"foreignKey:OrganizationID"`
	Members      []TeamUser    `json:"members,omitempty" gorm:"foreignKey:TeamID"`
}

// TableName specifies the table name
func (Team) TableName() string {
	return "teams"
}

// TeamUser represents a user's membership in a team
type TeamUser struct {
	ID        uint      `gorm:"primary_key" json:"id"`
	CreatedAt time.Time `json:"created_at"`

	TeamID             uint `json:"team_id" gorm:"not null;index;constraint:OnDelete:CASCADE"`
	OrganizationUserID uint `json:"organization_user_id" gorm:"not null;index;constraint:OnDelete:CASCADE"`

	// Role in team
	IsManager bool `json:"is_manager" gorm:"default:false"`

	// Associations
	Team             *Team             `json:"team,omitempty" gorm:"foreignKey:TeamID"`
	OrganizationUser *OrganizationUser `json:"organization_user,omitempty" gorm:"foreignKey:OrganizationUserID"`
}

// TableName specifies the table name
func (TeamUser) TableName() string {
	return "team_users"
}

// TeamDTO for API responses
type TeamDTO struct {
	ID                   uint      `json:"id"`
	UUID                 uuid.UUID `json:"uuid"`
	OrganizationID       uint      `json:"organization_id"`
	Name                 string    `json:"name"`
	Description          string    `json:"description,omitempty"`
	AccessAllCollections bool      `json:"access_all_collections"`
	ExternalID           *string   `json:"external_id,omitempty"`
	IsDefault            bool      `json:"is_default"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`

	// Stats (optional)
	MemberCount *int `json:"member_count,omitempty"`
}

// TeamUserDTO for API responses
type TeamUserDTO struct {
	ID                 uint      `json:"id"`
	TeamID             uint      `json:"team_id"`
	OrganizationUserID uint      `json:"organization_user_id"`
	UserID             uint      `json:"user_id"`
	UserEmail          string    `json:"user_email"`
	UserName           string    `json:"user_name"`
	IsManager          bool      `json:"is_manager"`
	CreatedAt          time.Time `json:"created_at"`
}

// CreateTeamRequest for API requests
type CreateTeamRequest struct {
	Name                 string  `json:"name" validate:"required,max=255"`
	Description          string  `json:"description,omitempty" validate:"max=1000"`
	AccessAllCollections bool    `json:"access_all_collections"`
	ExternalID           *string `json:"external_id,omitempty"`
}

// UpdateTeamRequest for API requests
type UpdateTeamRequest struct {
	Name                 *string `json:"name,omitempty" validate:"omitempty,max=255"`
	Description          *string `json:"description,omitempty" validate:"omitempty,max=1000"`
	AccessAllCollections *bool   `json:"access_all_collections,omitempty"`
}

// AddTeamUserRequest for adding users to team
type AddTeamUserRequest struct {
	OrganizationUserID uint `json:"organization_user_id" validate:"required"`
	IsManager          bool `json:"is_manager"`
}

// UpdateTeamUserRequest for updating team member
type UpdateTeamUserRequest struct {
	IsManager bool `json:"is_manager"`
}

// ToTeamDTO converts Team to DTO
func ToTeamDTO(team *Team) *TeamDTO {
	if team == nil {
		return nil
	}

	return &TeamDTO{
		ID:                   team.ID,
		UUID:                 team.UUID,
		OrganizationID:       team.OrganizationID,
		Name:                 team.Name,
		Description:          team.Description,
		AccessAllCollections: team.AccessAllCollections,
		ExternalID:           team.ExternalID,
		IsDefault:            team.IsDefault,
		CreatedAt:            team.CreatedAt,
		UpdatedAt:            team.UpdatedAt,
	}
}

// ToTeamUserDTO converts TeamUser to DTO
func ToTeamUserDTO(tu *TeamUser) *TeamUserDTO {
	if tu == nil {
		return nil
	}

	dto := &TeamUserDTO{
		ID:                 tu.ID,
		TeamID:             tu.TeamID,
		OrganizationUserID: tu.OrganizationUserID,
		IsManager:          tu.IsManager,
		CreatedAt:          tu.CreatedAt,
	}

	// Add user info if loaded
	if tu.OrganizationUser != nil && tu.OrganizationUser.User != nil {
		dto.UserID = tu.OrganizationUser.UserID
		dto.UserEmail = tu.OrganizationUser.User.Email
		dto.UserName = tu.OrganizationUser.User.Name
	}

	return dto
}

