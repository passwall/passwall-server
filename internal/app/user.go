package app

import (
	"github.com/pass-wall/passwall-server/internal/storage"
	"github.com/pass-wall/passwall-server/model"
)

// CreateUser creates a user and saves it to the store
func CreateUser(s storage.Store, dto *model.UserDTO) (*model.User, error) {

	createdUser, err := s.Users().Save(model.ToUser(dto))
	if err != nil {
		return nil, err
	}

	return createdUser, nil
}

// UpdateUser updates the user with the dto and applies the changes in the store
func UpdateUser(s storage.Store, user *model.User, userDTO *model.UserDTO, isAuthorized bool) (*model.User, error) {
	if userDTO.MasterPassword != "" {
		userDTO.MasterPassword = NewSHA256([]byte(userDTO.MasterPassword))
	} else {
		userDTO.MasterPassword = user.MasterPassword
	}

	user.Name = userDTO.Name
	user.Email = userDTO.Email
	user.MasterPassword = userDTO.MasterPassword
	user.Schema = userDTO.Schema

	// Only Admin's can change plan and role
	if isAuthorized {
		user.Plan = userDTO.Plan
		user.Role = userDTO.Role
	}

	updatedUser, err := s.Users().Save(user)
	if err != nil {
		return nil, err
	}
	return updatedUser, nil
}

// Migrate ...
func Migrate(s storage.Store, schema string) error {
	return s.Users().Migrate(schema)
}
