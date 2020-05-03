package app

import (
	"encoding/base64"

	"github.com/pass-wall/passwall-server/internal/storage"
	"github.com/pass-wall/passwall-server/model"
	"github.com/spf13/viper"
)

// CreateBankAccount creates a new bank account and saves it to the store
func CreateBankAccount(s storage.Store, dto *model.BankAccountDTO) (*model.BankAccount, error) {
	if dto.Password == "" {
		dto.Password = Password()
	}

	rawPass := dto.Password
	dto.Password = base64.StdEncoding.EncodeToString(Encrypt(dto.Password, viper.GetString("server.passphrase")))

	createdBankAccount, err := s.BankAccounts().Save(*model.ToBankAccount(dto))
	if err != nil {
		return nil, err
	}

	createdBankAccount.Password = rawPass

	return &createdBankAccount, nil
}

// UpdateBankAccount updates the account with the dto and applies the changes in the store
func UpdateBankAccount(s storage.Store, account *model.BankAccount, dto *model.BankAccountDTO) (*model.BankAccount, error) {
	if dto.Password == "" {
		dto.Password = Password()
	}
	rawPass := dto.Password
	dto.Password = base64.StdEncoding.EncodeToString(Encrypt(dto.Password, viper.GetString("server.passphrase")))

	dto.ID = uint(account.ID)
	bankAccount := model.ToBankAccount(dto)
	bankAccount.ID = uint(account.ID)

	updatedBankAccount, err := s.BankAccounts().Save(*bankAccount)
	if err != nil {

		return nil, err
	}

	updatedBankAccount.Password = rawPass
	return &updatedBankAccount, nil
}
