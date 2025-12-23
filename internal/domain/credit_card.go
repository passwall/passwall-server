package domain

import "time"

// CreditCard represents stored credit card information
type CreditCard struct {
	ID                 uint       `gorm:"primary_key" json:"id"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
	DeletedAt          *time.Time `json:"deleted_at,omitempty" gorm:"index"`
	CardName           string     `json:"title" gorm:"type:varchar(255)"`
	CardholderName     string     `json:"cardholder_name" gorm:"type:text" encrypt:"true"`
	Type               string     `json:"type" gorm:"type:varchar(50)" encrypt:"true"`
	Number             string     `json:"number" gorm:"type:text" encrypt:"true"`
	VerificationNumber string     `json:"verification_number" gorm:"type:varchar(10)" encrypt:"true"`
	ExpiryDate         string     `json:"expiry_date" gorm:"type:varchar(10)" encrypt:"true"`
}

// TableName specifies the table name for CreditCard
func (CreditCard) TableName() string {
	return "credit_cards"
}

