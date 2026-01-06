package model

import (
	"time"
)

// CreditCard ...
type CreditCard struct {
	ID                 uint       `gorm:"primary_key" json:"id"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
	DeletedAt          *time.Time `json:"deleted_at"`
	CardName           string     `json:"title"`
	CardholderName     string     `json:"cardholder_name" encrypt:"true"`
	Type               string     `json:"type" encrypt:"true"`
	Number             string     `json:"number" encrypt:"true"`
	VerificationNumber string     `json:"verification_number" encrypt:"true"`
	ExpiryDate         string     `json:"expiry_date" encrypt:"true"`
}

// CreditCardDTO DTO object for CreditCard type
type CreditCardDTO struct {
	ID                 uint   `json:"id"`
	CardName           string `json:"title"`
	CardholderName     string `json:"cardholder_name"`
	Type               string `json:"type"`
	Number             string `json:"number"`
	VerificationNumber string `json:"verification_number"`
	ExpiryDate         string `json:"expiry_date"`
}

// ToCreditCard ...
func ToCreditCard(creditCardDTO *CreditCardDTO) *CreditCard {
	return &CreditCard{
		CardName:           creditCardDTO.CardName,
		CardholderName:     creditCardDTO.CardholderName,
		Type:               creditCardDTO.Type,
		Number:             creditCardDTO.Number,
		VerificationNumber: creditCardDTO.VerificationNumber,
		ExpiryDate:         creditCardDTO.ExpiryDate,
	}
}

// ToCreditCardDTO ...
func ToCreditCardDTO(creditCard *CreditCard) *CreditCardDTO {
	return &CreditCardDTO{
		ID:                 creditCard.ID,
		CardName:           creditCard.CardName,
		CardholderName:     creditCard.CardholderName,
		Type:               creditCard.Type,
		Number:             creditCard.Number,
		VerificationNumber: creditCard.VerificationNumber,
		ExpiryDate:         creditCard.ExpiryDate,
	}
}

// ToCreditCardDTOs ...
func ToCreditCardDTOs(creditCards []*CreditCard) []*CreditCardDTO {
	creditCardDTOs := make([]*CreditCardDTO, len(creditCards))

	for i, itm := range creditCards {
		creditCardDTOs[i] = ToCreditCardDTO(itm)
	}

	return creditCardDTOs
}

/* EXAMPLE JSON OBJECT
{
	"card_name":"Bank Bonus",
	"cardholder_name": "John Doe",
	"type": "Matercard",
	"number": "1234-5678-1234-5678",
	"verification_number": "000",
	"expiry_date": "12/2022"
}
*/
