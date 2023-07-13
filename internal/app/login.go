package app

import (
	"github.com/passwall/passwall-server/internal/storage"
	"github.com/passwall/passwall-server/model"
	"github.com/passwall/passwall-server/pkg/logger"
)

// FindAllLogins finds all logins
func FindAllLogins(s storage.Store, schema string) ([]model.Login, error) {
	loginList, err := s.Logins().All(schema)
	if err != nil {
		return nil, err
	}

	// Decrypt server side encrypted fields
	for i := range loginList {
		uLogin, err := DecryptModel(&loginList[i])
		if err != nil {
			logger.Errorf("Error while decrypting login: %v", err)
			continue
		}
		loginList[i] = *uLogin.(*model.Login)
	}

	return loginList, nil
}

// CreateLogin creates a login and saves it to the store
func CreateLogin(s storage.Store, dto *model.LoginDTO, schema string) (*model.Login, error) {
	rawLogin := model.ToLogin(dto)
	encLogin := EncryptModel(rawLogin)

	createdLogin, err := s.Logins().Create(encLogin.(*model.Login), schema)
	if err != nil {
		return nil, err
	}

	return createdLogin, nil
}

// CreateLogins is needed for import
func CreateLogins(s storage.Store, dtos []model.LoginDTO, schema string) error {
	for i := range dtos {
		rawLogin := model.ToLogin(&dtos[i])
		encLogin := EncryptModel(rawLogin)

		_, err := s.Logins().Create(encLogin.(*model.Login), schema)
		if err != nil {
			return err
		}
	}

	return nil
}

// UpdateLogin updates the login with the dto and applies the changes in the store
func UpdateLogin(s storage.Store, login *model.Login, dto *model.LoginDTO, schema string) (*model.Login, error) {
	rawModel := model.ToLogin(dto)
	encModel := EncryptModel(rawModel).(*model.Login)

	login.Title = encModel.Title
	login.URL = encModel.URL
	login.Username = encModel.Username
	login.Password = encModel.Password
	login.Extra = encModel.Extra
	login.TOTPSecret = encModel.TOTPSecret

	updatedLogin, err := s.Logins().Update(login, schema)
	if err != nil {
		return nil, err
	}

	return updatedLogin, nil
}
