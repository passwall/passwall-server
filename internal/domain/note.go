package domain

import "time"

// Note represents a secure note
type Note struct {
	ID        uint       `gorm:"primary_key" json:"id"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty" gorm:"index"`
	Title     string     `json:"title" gorm:"type:varchar(255)"`
	Note      string     `json:"note" gorm:"type:text" encrypt:"true"`
}

// TableName specifies the table name for Note
func (Note) TableName() string {
	return "notes"
}

