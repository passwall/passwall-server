package domain

import (
	"time"

	uuid "github.com/satori/go.uuid"
)

// UserDTO is the data transfer object for User
// It converts the Role relationship to a simple string for API responses
type UserDTO struct {
	ID           uint       `json:"id"`
	UUID         uuid.UUID  `json:"uuid"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	Name         string     `json:"name"`
	Email        string     `json:"email"`
	Schema       string     `json:"schema"`
	Role         string     `json:"role"` // Role name as string for backward compatibility
	IsVerified   bool       `json:"is_verified"`
	LastSignInAt *time.Time `json:"last_sign_in_at"`
	IsMigrated   bool       `json:"is_migrated"`
	DateOfBirth  *time.Time `json:"date_of_birth,omitempty"`
	Language     string     `json:"language"`
}

// ToUserDTO converts User to UserDTO
func ToUserDTO(user *User) *UserDTO {
	if user == nil {
		return nil
	}

	return &UserDTO{
		ID:           user.ID,
		UUID:         user.UUID,
		CreatedAt:    user.CreatedAt,
		UpdatedAt:    user.UpdatedAt,
		Name:         user.Name,
		Email:        user.Email,
		Schema:       user.Schema,
		Role:         user.GetRoleName(),
		IsVerified:   user.IsVerified,
		LastSignInAt: user.LastSignInAt,
		IsMigrated:   user.IsMigrated,
		DateOfBirth:  user.DateOfBirth,
		Language:     user.Language,
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
