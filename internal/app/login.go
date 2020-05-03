package app

import (
	"encoding/base64"

	"github.com/pass-wall/passwall-server/internal/storage"
	"github.com/pass-wall/passwall-server/model"
	"github.com/spf13/viper"
)

// CreateLogin creates a login and saves it to the store
func CreateLogin(s storage.Store, dto *model.LoginDTO) (*model.Login, error) {

	rawPass := dto.Password
	dto.Password = base64.StdEncoding.EncodeToString(Encrypt(dto.Password, viper.GetString("server.passphrase")))

	createdLogin, err := s.Logins().Save(*model.ToLogin(dto))
	if err != nil {
		return nil, err
	}

	createdLogin.Password = rawPass
	return &createdLogin, nil

}

// UpdateLogin updates the login with the dto and applies the changes in the store
func UpdateLogin(s storage.Store, login *model.Login, dto *model.LoginDTO) (*model.Login, error) {

	if dto.Password == "" {
		dto.Password = Password()
	}
	rawPass := dto.Password
	dto.Password = base64.StdEncoding.EncodeToString(Encrypt(dto.Password, viper.GetString("server.passphrase")))

	login.URL = dto.URL
	login.Username = dto.Username
	login.Password = dto.Password
	updatedLogin, err := s.Logins().Save(*login)
	if err != nil {
		return nil, err
	}
	updatedLogin.Password = rawPass
	return &updatedLogin, nil
}
