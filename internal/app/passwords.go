package app

import (
	"encoding/base64"
	"net/http"

	"github.com/pass-wall/passwall-server/internal/common"
	"github.com/pass-wall/passwall-server/internal/encryption"
	"github.com/pass-wall/passwall-server/model"
	"github.com/spf13/viper"
)

// GeneratePassword generates new password
func GeneratePassword(w http.ResponseWriter, r *http.Request) {
	password := encryption.Password()
	response := model.Response{Code: http.StatusOK, Status: Success, Message: password}
	common.RespondWithJSON(w, http.StatusOK, response)
}

// FindSamePassword ...
func FindSamePassword(p *LoginService, password model.Password) (model.URLs, error) {

	loginList, err := p.LoginRepository.All()

	loginList = DecryptLoginPasswords(loginList)

	newUrls := model.URLs{Items: []string{}}

	for _, login := range loginList {
		if login.Password == password.Password {
			newUrls.AddItem(login.URL)
		}
	}

	return newUrls, err
}

// DecryptLoginPasswords ...
func DecryptLoginPasswords(loginList []model.Login) []model.Login {
	for i := range loginList {
		if loginList[i].Password == "" {
			continue
		}
		passByte, _ := base64.StdEncoding.DecodeString(loginList[i].Password)
		passB64 := string(encryption.Decrypt(string(passByte[:]), viper.GetString("server.passphrase")))
		loginList[i].Password = passB64
	}
	return loginList
}

// DecryptBankAccountPasswords ...
func DecryptBankAccountPasswords(bankAccounts []model.BankAccount) []model.BankAccount {
	for i := range bankAccounts {
		if bankAccounts[i].Password == "" {
			continue
		}
		passByte, _ := base64.StdEncoding.DecodeString(bankAccounts[i].Password)
		passB64 := string(encryption.Decrypt(string(passByte[:]), viper.GetString("server.passphrase")))
		bankAccounts[i].Password = passB64
	}
	return bankAccounts
}

// DecryptCreditCardVerificationNumbers ...
func DecryptCreditCardVerificationNumbers(creditCards []model.CreditCard) []model.CreditCard {
	for i := range creditCards {
		if creditCards[i].VerificationNumber == "" {
			continue
		}
		passByte, _ := base64.StdEncoding.DecodeString(creditCards[i].VerificationNumber)
		passB64 := string(encryption.Decrypt(string(passByte[:]), viper.GetString("server.passphrase")))
		creditCards[i].VerificationNumber = passB64
	}
	return creditCards
}
