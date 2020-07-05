package app

import (
	"encoding/base64"

	"github.com/passwall/passwall-server/internal/storage"
	"github.com/passwall/passwall-server/model"
	"github.com/spf13/viper"
)

// CreateCreditCard creates a new credit card and saves it to the store
func CreateCreditCard(s storage.Store, dto *model.CreditCardDTO, schema string) (*model.CreditCard, error) {

	rawPass := dto.VerificationNumber
	dto.VerificationNumber = base64.StdEncoding.EncodeToString(Encrypt(dto.VerificationNumber, viper.GetString("server.passphrase")))

	createdCreditCard, err := s.CreditCards().Save(model.ToCreditCard(dto), schema)
	if err != nil {
		return nil, err
	}

	createdCreditCard.VerificationNumber = rawPass

	return createdCreditCard, nil

}

// UpdateCreditCard updates the credit card with the dto and applies the changes in the store
func UpdateCreditCard(s storage.Store, card *model.CreditCard, dto *model.CreditCardDTO, schema string) (*model.CreditCard, error) {
	rawPass := dto.VerificationNumber
	dto.VerificationNumber = base64.StdEncoding.EncodeToString(Encrypt(dto.VerificationNumber, viper.GetString("server.passphrase")))

	dto.ID = uint(card.ID)
	creditCard := model.ToCreditCard(dto)
	creditCard.ID = uint(card.ID)

	updatedCreditCard, err := s.CreditCards().Save(creditCard, schema)
	if err != nil {
		return nil, err
	}
	updatedCreditCard.VerificationNumber = rawPass
	return updatedCreditCard, nil
}

// DecryptCreditCardVerificationNumber decrypts verification number
func DecryptCreditCardVerificationNumber(s storage.Store, card *model.CreditCard) (*model.CreditCard, error) {
	passByte, _ := base64.StdEncoding.DecodeString(card.VerificationNumber)
	card.VerificationNumber = string(Decrypt(string(passByte[:]), viper.GetString("server.passphrase")))

	return card, nil
}

// DecryptCreditCardVerificationNumbers ...
// TODO: convert to pointers
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
