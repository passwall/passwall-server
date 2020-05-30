package app

import (
	"encoding/base64"

	"github.com/pass-wall/passwall-server/internal/storage"
	"github.com/pass-wall/passwall-server/model"
	"github.com/spf13/viper"
)

// CreateEmail creates a new bank account and saves it to the store
func CreateEmail(s storage.Store, dto *model.EmailDTO) (*model.Email, error) {
	if dto.Password == "" {
		generatedPass, err := Password()
		if err != nil {
			return nil, err
		}
		dto.Password = generatedPass
	}

	rawPass := dto.Password
	dto.Password = base64.StdEncoding.EncodeToString(Encrypt(dto.Password, viper.GetString("server.passphrase")))

	createdEmail, err := s.Emails().Save(*model.ToEmail(dto))
	if err != nil {
		return nil, err
	}

	createdEmail.Password = rawPass

	return &createdEmail, nil
}

// UpdateEmail updates the account with the dto and applies the changes in the store
func UpdateEmail(s storage.Store, account *model.Email, dto *model.EmailDTO) (*model.Email, error) {
	if dto.Password == "" {
		generatedPass, err := Password()
		if err != nil {
			return nil, err
		}
		dto.Password = generatedPass
	}
	rawPass := dto.Password
	dto.Password = base64.StdEncoding.EncodeToString(Encrypt(dto.Password, viper.GetString("server.passphrase")))

	dto.ID = uint(account.ID)
	email := model.ToEmail(dto)
	email.ID = uint(account.ID)

	updatedEmail, err := s.Emails().Save(*email)
	if err != nil {

		return nil, err
	}

	updatedEmail.Password = rawPass
	return &updatedEmail, nil
}

// DecryptEmailPassword decrypts password
func DecryptEmailPassword(s storage.Store, account *model.Email) (*model.Email, error) {
	passByte, _ := base64.StdEncoding.DecodeString(account.Password)
	account.Password = string(Decrypt(string(passByte[:]), viper.GetString("server.passphrase")))

	return account, nil
}

// DecryptEmailPasswords ...
// TODO: convert to pointers
func DecryptEmailPasswords(emails []model.Email) []model.Email {
	for i := range emails {
		if emails[i].Password == "" {
			continue
		}
		passByte, _ := base64.StdEncoding.DecodeString(emails[i].Password)
		passB64 := string(Decrypt(string(passByte[:]), viper.GetString("server.passphrase")))
		emails[i].Password = passB64
	}
	return emails
}
