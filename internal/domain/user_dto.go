package domain

import (
	"errors"
	"time"

	uuid "github.com/satori/go.uuid"
)

// UserDTO is the data transfer object for User
// It converts the Role relationship to a simple string for API responses
type UserDTO struct {
	ID            uint      `json:"id"`
	UUID          uuid.UUID `json:"uuid"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	Name          string    `json:"name"`
	Email         string    `json:"email"`
	Schema        string    `json:"schema"`
	Role          string    `json:"role"`
	IsVerified    bool      `json:"is_verified"`
	IsSystemUser  bool      `json:"is_system_user"` // System users cannot be deleted
	Language      string    `json:"language"`
	KdfType       KdfType   `json:"kdf_type"`
	KdfIterations int       `json:"kdf_iterations"`
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

// CreateUserByAdminRequest represents admin-created user request (zero-knowledge)
type CreateUserByAdminRequest struct {
	Name               string     `json:"name" validate:"required,max=100"`
	Email              string     `json:"email" validate:"required,email"`
	MasterPasswordHash string     `json:"master_password_hash" validate:"required"` // HKDF(masterKey, info="auth")
	ProtectedUserKey   string     `json:"protected_user_key" validate:"required"`   // EncString: "2.iv|ct|mac"
	KdfConfig          *KdfConfig `json:"kdf_config" validate:"required"`
	KdfSalt            string     `json:"kdf_salt" validate:"required"` // hex-encoded random salt
	RoleID             *uint      `json:"role_id,omitempty"`
}

// Validate validates the create user request
func (r *CreateUserByAdminRequest) Validate() error {
	if r.Name == "" {
		return errors.New("name is required")
	}
	if r.Email == "" {
		return errors.New("email is required")
	}
	if r.MasterPasswordHash == "" {
		return errors.New("master password hash is required")
	}
	if r.ProtectedUserKey == "" {
		return errors.New("protected user key is required")
	}
	if r.KdfConfig == nil {
		return errors.New("KDF configuration is required")
	}
	if r.KdfSalt == "" {
		return errors.New("KDF salt is required")
	}

	// Validate KDF config
	if err := r.KdfConfig.Validate(); err != nil {
		return err
	}

	return nil
}

// UpdateUserRequest represents user update request
type UpdateUserRequest struct {
	Name        *string `json:"name,omitempty"`
	Email       *string `json:"email,omitempty"`
	RoleID      *uint   `json:"role_id,omitempty"`
	DateOfBirth *string `json:"date_of_birth,omitempty"`
	Language    *string `json:"language,omitempty"`
}

// HasUpdates checks if any field is being updated
func (r *UpdateUserRequest) HasUpdates() bool {
	return r.Name != nil || r.Email != nil || r.RoleID != nil || r.DateOfBirth != nil || r.Language != nil
}

// ApplyTo applies the update request to a user
func (r *UpdateUserRequest) ApplyTo(user *User) {
	if r.Name != nil {
		user.Name = *r.Name
	}
	if r.Email != nil {
		user.Email = *r.Email
	}
	if r.RoleID != nil {
		user.RoleID = *r.RoleID
	}
	if r.Language != nil {
		user.Language = *r.Language
	}
}
