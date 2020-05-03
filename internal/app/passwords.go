package app

import (
	"encoding/base64"

	"github.com/pass-wall/passwall-server/internal/storage"
	"github.com/pass-wall/passwall-server/model"
	"github.com/spf13/viper"
)

// FindSamePassword ...
func FindSamePassword(s storage.Store, password model.Password) (model.URLs, error) {

	logins, err := s.Logins().All()

	logins = DecryptLoginPasswords(logins)

	newUrls := model.URLs{Items: []string{}}

	for _, login := range logins {
		if login.Password == password.Password {
			newUrls.AddItem(login.URL)
		}
	}

	return newUrls, err
}

// DecryptLoginPasswords ...
func DecryptLoginPasswords(logins []model.Login) []model.Login {
	for i := range logins {
		if logins[i].Password == "" {
			continue
		}
		passByte, _ := base64.StdEncoding.DecodeString(logins[i].Password)
		passB64 := string(Decrypt(string(passByte[:]), viper.GetString("server.passphrase")))
		logins[i].Password = passB64
	}
	return logins
}

// DecryptBankAccountPasswords ...
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

// DecryptCreditCardVerificationNumbers ...
func DecryptCreditCardVerificationNumbers(creditCards []model.CreditCard) []model.CreditCard {
	for i := range creditCards {
		if creditCards[i].VerificationNumber == "" {
			continue
		}
		passByte, _ := base64.StdEncoding.DecodeString(creditCards[i].VerificationNumber)
		passB64 := string(Decrypt(string(passByte[:]), viper.GetString("server.passphrase")))
		creditCards[i].VerificationNumber = passB64
	}
	return creditCards
}

// DecryptNotes ...
func DecryptNotes(notes []model.Note) []model.Note {
	for i := range notes {
		if notes[i].Note == "" {
			continue
		}
		passByte, _ := base64.StdEncoding.DecodeString(notes[i].Note)
		passB64 := string(Decrypt(string(passByte[:]), viper.GetString("server.passphrase")))
		notes[i].Note = passB64
	}
	return notes
}
