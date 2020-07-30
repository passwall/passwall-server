package app

import (
	"encoding/base64"

	"github.com/passwall/passwall-server/internal/storage"
	"github.com/passwall/passwall-server/model"
	"github.com/spf13/viper"
)

// CreateBankAccount creates a new bank account and saves it to the store
func CreateBankAccount(s storage.Store, dto *model.BankAccountDTO, schema string) (*model.BankAccount, error) {
	rawModel := model.ToBankAccount(dto)
	encModel := EncryptModel(rawModel)

	createdBankAccount, err := s.BankAccounts().Save(encModel.(*model.BankAccount), schema)
	if err != nil {
		return nil, err
	}

	return createdBankAccount, nil
}

// UpdateBankAccount updates the account with the dto and applies the changes in the store
func UpdateBankAccount(s storage.Store, bankAccount *model.BankAccount, dto *model.BankAccountDTO, schema string) (*model.BankAccount, error) {
	rawModel := model.ToBankAccount(dto)
	encModel := EncryptModel(rawModel).(*model.BankAccount)

	bankAccount.BankName = encModel.BankName
	bankAccount.BankCode = encModel.BankCode
	bankAccount.AccountName = encModel.AccountName
	bankAccount.AccountNumber = encModel.AccountNumber
	bankAccount.IBAN = encModel.IBAN
	bankAccount.Currency = encModel.Currency
	bankAccount.Password = encModel.Password

	updatedBankAccount, err := s.BankAccounts().Save(bankAccount, schema)
	if err != nil {
		return nil, err
	}

	return updatedBankAccount, nil
}

// DecryptBankAccountPassword decrypts password
func DecryptBankAccountPassword(s storage.Store, account *model.BankAccount) (*model.BankAccount, error) {
	passByte, _ := base64.StdEncoding.DecodeString(account.Password)
	account.Password = string(Decrypt(string(passByte[:]), viper.GetString("server.passphrase")))

	return account, nil
}

// DecryptBankAccountPasswords ...
// TODO: convert to pointers
func DecryptBankAccountPasswords(bankAccounts []model.BankAccount) []model.BankAccount {
	for i := range bankAccounts {
		if bankAccounts[i].Password == "" {
			continue
		}
		passByte, _ := base64.StdEncoding.DecodeString(bankAccounts[i].Password)
		passB64 := string(Decrypt(string(passByte[:]), viper.GetString("server.passphrase")))
		bankAccounts[i].Password = passB64
	}
	return bankAccounts
}
