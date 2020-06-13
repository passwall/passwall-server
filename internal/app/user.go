package app

import (
	"fmt"

	"github.com/pass-wall/passwall-server/internal/storage"
	"github.com/pass-wall/passwall-server/model"
	uuid "github.com/satori/go.uuid"
)

// CreateUser creates a user and saves it to the store
func CreateUser(s storage.Store, userDTO *model.UserDTO) (*model.User, error) {

	// Hasing the master password with Bcrypt
	userDTO.MasterPassword = NewBcrypt([]byte(userDTO.MasterPassword))

	// Generate secret to use as salt
	userDTO.Secret = GenerateSecureKey(16)

	// New user's plan is Free and role is Member (not Admin)
	userDTO.Plan = "Free"
	userDTO.Role = "Member"

	// Generate new UUID for user
	userDTO.UUID = uuid.NewV4()

	createdUser, err := s.Users().Save(model.ToUser(userDTO))
	if err != nil {
		return nil, err
	}

	return createdUser, nil
}

// UpdateUser updates the user with the dto and applies the changes in the store
func UpdateUser(s storage.Store, user *model.User, userDTO *model.UserDTO, isAuthorized bool) (*model.User, error) {

	// TODO: Refactor the contents of updated user with a logical way
	if userDTO.MasterPassword != "" && NewBcrypt([]byte(userDTO.MasterPassword)) != user.MasterPassword {
		userDTO.MasterPassword = NewBcrypt([]byte(userDTO.MasterPassword))
	} else {
		userDTO.MasterPassword = user.MasterPassword
	}

	user.Name = userDTO.Name
	user.Email = userDTO.Email
	user.MasterPassword = userDTO.MasterPassword

	// This never changes
	user.Schema = fmt.Sprintf("user%d", user.ID)

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

// GenerateSchema creates user schema and tables
func GenerateSchema(s storage.Store, user *model.User) (*model.User, error) {
	user.Schema = fmt.Sprintf("user%d", user.ID)
	savedUser, err := s.Users().Save(user)
	if err != nil {
		return nil, err
	}
	return savedUser, nil
}
