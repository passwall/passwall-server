package app

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/spf13/viper"

	"github.com/passwall/passwall-server/internal/storage"
	"github.com/passwall/passwall-server/model"
	"github.com/passwall/passwall-server/pkg/logger"
	uuid "github.com/satori/go.uuid"
)

var (
	// ErrGenerateSchema represents message for generating schema
	ErrGenerateSchema = errors.New("an error occured while genarating schema")
	// ErrCreateSchema represents message for creating schema
	ErrCreateSchema = errors.New("an error occured while creating the schema and tables")
)

// CreateUser creates a user and saves it to the store
func CreateUser(s storage.Store, userDTO *model.UserDTO) (*model.User, error) {
	var err error

	//Run validator according to model.UserDTO validator tags
	err = PayloadValidator(userDTO)
	if err != nil {
		logger.Errorf("Error while validating userDTO: %v", err)
		return nil, err
	}

	// Hashing the master password with Bcrypt
	userDTO.MasterPassword = NewBcrypt([]byte(userDTO.MasterPassword))

	passwordLength, err := strconv.Atoi(viper.GetString("server.generatedPasswordLength"))
	if err != nil {
		logger.Errorf("Error while converting passwordLength: %v", err)
		return nil, err
	}

	userDTO.Secret, err = GenerateSecureKey(passwordLength)
	if err != nil {
		logger.Errorf("Error while generating secure key: %v", err)
		return nil, err
	}

	// New user's role is Member (not Admin)
	userDTO.Role = "Member"

	// Generate new UUID for user
	userDTO.UUID = uuid.NewV4()

	createdUser, err := s.Users().Create(model.ToUser(userDTO))
	if err != nil {
		logger.Errorf("Error while creating user: %v", err)
		return nil, err
	}

	confirmationCode := RandomMD5Hash()
	createdUser.ConfirmationCode = confirmationCode

	// Generate schema name and update user
	updatedUser, err := GenerateSchema(s, createdUser)
	if err != nil {
		logger.Errorf("Error while generating schema: %v", err)
		return nil, ErrGenerateSchema
	}

	// Create user schema and tables
	err = s.Users().CreateSchema(updatedUser.Schema)
	if err != nil {
		logger.Errorf("Error while creating schema: %v", err)
		return nil, ErrCreateSchema
	}

	// Create user tables in user schema
	err = MigrateUserTables(s, updatedUser.Schema)
	if err != nil {
		logger.Errorf("Error while migrating user tables: %v", err)
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

	updatedUser, err := s.Users().Update(user)
	if err != nil {
		return nil, err
	}
	return updatedUser, nil
}

// ChangeMasterPassword updates the user with the new master password
func ChangeMasterPassword(s storage.Store, user *model.User, newMasterPassword string) (*model.User, error) {
	user.MasterPassword = NewBcrypt([]byte(newMasterPassword))
	updatedUser, err := s.Users().Update(user)
	if err != nil {
		return nil, err
	}
	return updatedUser, nil
}

// GenerateSchema creates user schema and tables
func GenerateSchema(s storage.Store, user *model.User) (*model.User, error) {
	user.Schema = fmt.Sprintf("user%d", user.ID)
	savedUser, err := s.Users().Update(user)
	if err != nil {
		logger.Errorf("Error while updating user schema: %v", err)
		return nil, err
	}

	return savedUser, nil
}
