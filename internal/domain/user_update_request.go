package domain

import "time"

// UpdateUserRequest represents the request body for updating a user
type UpdateUserRequest struct {
	Name        *string    `json:"name,omitempty"`
	Email       *string    `json:"email,omitempty"`
	RoleID      *uint      `json:"role_id,omitempty"`
	DateOfBirth *time.Time `json:"date_of_birth,omitempty"`
	Language    *string    `json:"language,omitempty"`
}

// HasUpdates checks if any field is set for update
func (r *UpdateUserRequest) HasUpdates() bool {
	return r.Name != nil || r.Email != nil || r.RoleID != nil || r.DateOfBirth != nil || r.Language != nil
}

// ApplyTo applies the updates to a user entity
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
	if r.DateOfBirth != nil {
		user.DateOfBirth = r.DateOfBirth
	}
	if r.Language != nil {
		user.Language = *r.Language
	}
}

