package app

import (
	"fmt"
	"strconv"

	"github.com/spf13/viper"

	"github.com/passwall/passwall-server/internal/storage"
	"github.com/passwall/passwall-server/model"
	uuid "github.com/satori/go.uuid"
)

var (
	//ErrGenerateSchema represents message for generating schema
	ErrGenerateSchema = fmt.Errorf("an error occured while genarating schema")
	//ErrCreateSchema represents message for creating schema
	ErrCreateSchema = fmt.Errorf("an error occured while creating the schema and tables")
)

// CreateUser creates a user and saves it to the store
func CreateUser(s storage.Store, userDTO *model.UserDTO) (*model.User, error) {
	var err error
	// Hashing the master password with Bcrypt
	userDTO.MasterPassword = NewBcrypt([]byte(userDTO.MasterPassword))

	passwordLength, _ := strconv.Atoi(viper.GetString("server.generatedPasswordLength"))
	userDTO.Secret, err = GenerateSecureKey(passwordLength)
	if err != nil {
		return nil, err
	}
	// New user's role is Member (not Admin)
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
	user.EmailVerifiedAt = userDTO.EmailVerifiedAt
	// This never changes
	user.Schema = fmt.Sprintf("user%d", user.ID)

	// Only Admin's can change role
	if isAuthorized {
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
