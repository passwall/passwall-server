package domain

import "time"

// Login represents stored login credentials
type Login struct {
	ID         uint       `gorm:"primary_key" json:"id"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	DeletedAt  *time.Time `json:"deleted_at,omitempty" gorm:"index"`
	Title      string     `json:"title" gorm:"type:varchar(255)"`
	URL        string     `json:"url" gorm:"type:text"`
	Username   string     `json:"username" gorm:"type:text" encrypt:"true"`
	Password   string     `json:"password" gorm:"type:text" encrypt:"true"`
	TOTPSecret string     `json:"totp_secret" gorm:"type:text" encrypt:"true"`
	Extra      string     `json:"extra" gorm:"type:text" encrypt:"true"`
}

// TableName specifies the table name for Login
func (Login) TableName() string {
	return "logins"
}

