package model

import (
	"time"
)

// BankAccount ...
type BankAccount struct {
	ID            uint       `gorm:"primary_key" json:"id"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	DeletedAt     *time.Time `json:"deleted_at"`
	BankName      string     `json:"bank_name"`
	BankCode      string     `json:"bank_code"`
	AccountName   string     `json:"account_name" encrypt:"true"`
	AccountNumber string     `json:"account_number" encrypt:"true"`
	IBAN          string     `json:"iban" encrypt:"true"`
	Currency      string     `json:"currency" encrypt:"true"`
	Password      string     `json:"password" encrypt:"true"`
}

//BankAccountDTO DTO object for BankAccount type
type BankAccountDTO struct {
	ID            uint   `json:"id"`
	BankName      string `json:"bank_name"`
	BankCode      string `json:"bank_code"`
	AccountName   string `json:"account_name"`
	AccountNumber string `json:"account_number"`
	IBAN          string `json:"iban"`
	Currency      string `json:"currency"`
	Password      string `json:"password"`
}

// ToBankAccount ...
func ToBankAccount(bankAccountDTO *BankAccountDTO) *BankAccount {
	return &BankAccount{
		BankName:      bankAccountDTO.BankName,
		BankCode:      bankAccountDTO.BankCode,
		AccountName:   bankAccountDTO.AccountName,
		AccountNumber: bankAccountDTO.AccountNumber,
		IBAN:          bankAccountDTO.IBAN,
		Currency:      bankAccountDTO.Currency,
		Password:      bankAccountDTO.Password,
	}
}

// ToBankAccountDTO ...
func ToBankAccountDTO(bankAccount *BankAccount) *BankAccountDTO {
	return &BankAccountDTO{
		ID:            bankAccount.ID,
		BankName:      bankAccount.BankName,
		BankCode:      bankAccount.BankCode,
		AccountName:   bankAccount.AccountName,
		AccountNumber: bankAccount.AccountNumber,
		IBAN:          bankAccount.IBAN,
		Currency:      bankAccount.Currency,
		Password:      bankAccount.Password,
	}
}

// ToBankAccountDTOs ...
func ToBankAccountDTOs(bankAccounts []*BankAccount) []*BankAccountDTO {
	bankAccountDTOs := make([]*BankAccountDTO, len(bankAccounts))

	for i, itm := range bankAccounts {
		bankAccountDTOs[i] = ToBankAccountDTO(itm)
	}

	return bankAccountDTOs
}

/* EXAMPLE JSON OBJECT
{
	"bank_name":"Bank of Dummy",
	"bank_code": "DUMMY 12345",
	"account_name": "John Doe",
	"account_number": "123-456-789",
	"iban": "TR12 3456 7890 1234 5678 9012 34",
	"currency": "TL",
	"password": "dummypassword"
}
*/
