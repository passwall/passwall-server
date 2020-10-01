package app

import (
	"github.com/passwall/passwall-server/internal/storage"
	"github.com/passwall/passwall-server/model"
)

// CreateLogin creates a login and saves it to the store
func CreateLogin(s storage.Store, dto *model.LoginDTO, schema string) (*model.Login, error) {
	rawLogin := model.ToLogin(dto)
	encLogin := EncryptModel(rawLogin)

	createdLogin, err := s.Logins().Save(encLogin.(*model.Login), schema)
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

		_, err := s.Logins().Save(encLogin.(*model.Login), schema)
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

	updatedLogin, err := s.Logins().Save(login, schema)
	if err != nil {
		return nil, err
	}

	return updatedLogin, nil
}
