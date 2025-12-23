package domain

import "time"

// BankAccount represents stored bank account credentials
type BankAccount struct {
	ID            uint       `gorm:"primary_key" json:"id"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	DeletedAt     *time.Time `json:"deleted_at,omitempty" gorm:"index"`
	BankName      string     `json:"title" gorm:"type:varchar(255)"`
	BankCode      string     `json:"bank_code" gorm:"type:varchar(100)"`
	AccountName   string     `json:"account_name" gorm:"type:text" encrypt:"true"`
	AccountNumber string     `json:"account_number" gorm:"type:text" encrypt:"true"`
	IBAN          string     `json:"iban" gorm:"type:text" encrypt:"true"`
	Currency      string     `json:"currency" gorm:"type:varchar(10)" encrypt:"true"`
	Password      string     `json:"password" gorm:"type:text" encrypt:"true"`
}

// TableName specifies the table name for BankAccount
func (BankAccount) TableName() string {
	return "bank_accounts"
}

