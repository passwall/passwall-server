package domain

import "time"

// Email represents stored email credentials
type Email struct {
	ID        uint       `gorm:"primary_key" json:"id"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty" gorm:"index"`
	Title     string     `json:"title" gorm:"type:varchar(255)"`
	Email     string     `json:"email" gorm:"type:text" encrypt:"true"`
	Password  string     `json:"password" gorm:"type:text" encrypt:"true"`
}

// TableName specifies the table name for Email
func (Email) TableName() string {
	return "emails"
}

