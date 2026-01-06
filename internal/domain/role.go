package domain

import "time"

// Role represents a user role in the system
type Role struct {
	ID          uint         `gorm:"primary_key" json:"id"`
	Name        string       `json:"name" gorm:"type:varchar(50);uniqueIndex;not null"`
	DisplayName string       `json:"display_name" gorm:"type:varchar(100);not null"`
	Description string       `json:"description" gorm:"type:text"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
	Permissions []Permission `gorm:"many2many:role_permissions;" json:"permissions,omitempty"`
}

// TableName specifies the table name for Role
func (Role) TableName() string {
	return "roles"
}
