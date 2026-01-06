package domain

import (
	"time"

	uuid "github.com/satori/go.uuid"
)

// UserDTO is the data transfer object for User
// It converts the Role relationship to a simple string for API responses
type UserDTO struct {
	ID           uint      `json:"id"`
	UUID         uuid.UUID `json:"uuid"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	Name         string    `json:"name"`
	Email        string    `json:"email"`
	Schema       string    `json:"schema"`
	Role         string    `json:"role"`
	IsVerified   bool      `json:"is_verified"`
	IsSystemUser bool      `json:"is_system_user"` // System users cannot be deleted
	Language     string    `json:"language"`
	KdfType      KdfType   `json:"kdf_type"`
	KdfIterations int      `json:"kdf_iterations"`
}

// ToUserDTO converts User to UserDTO
func ToUserDTO(user *User) *UserDTO {
	if user == nil {
		return nil
	}

	return &UserDTO{
		ID:            user.ID,
		UUID:          user.UUID,
		CreatedAt:     user.CreatedAt,
		UpdatedAt:     user.UpdatedAt,
		Name:          user.Name,
		Email:         user.Email,
		Schema:        user.Schema,
		Role:          user.GetRoleName(),
		IsVerified:    user.IsVerified,
		IsSystemUser:  user.IsSystemUser,
		Language:      user.Language,
		KdfType:       user.KdfType,
		KdfIterations: user.KdfIterations,
	}
}

// ToUserDTOs converts multiple Users to UserDTOs
func ToUserDTOs(users []*User) []*UserDTO {
	dtos := make([]*UserDTO, len(users))
	for i, user := range users {
		dtos[i] = ToUserDTO(user)
	}
	return dtos
}
