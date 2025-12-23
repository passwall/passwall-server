package domain

import "time"

// Server represents stored server credentials
type Server struct {
	ID              uint       `gorm:"primary_key" json:"id"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	DeletedAt       *time.Time `json:"deleted_at,omitempty" gorm:"index"`
	Title           string     `json:"title" gorm:"type:varchar(255)"`
	IP              string     `json:"ip" gorm:"type:text" encrypt:"true"`
	Username        string     `json:"username" gorm:"type:text" encrypt:"true"`
	Password        string     `json:"password" gorm:"type:text" encrypt:"true"`
	URL             string     `json:"url" gorm:"type:text"`
	HostingUsername string     `json:"hosting_username" gorm:"type:text" encrypt:"true"`
	HostingPassword string     `json:"hosting_password" gorm:"type:text" encrypt:"true"`
	AdminUsername   string     `json:"admin_username" gorm:"type:text" encrypt:"true"`
	AdminPassword   string     `json:"admin_password" gorm:"type:text" encrypt:"true"`
	Extra           string     `json:"extra" gorm:"type:text" encrypt:"true"`
}

// TableName specifies the table name for Server
func (Server) TableName() string {
	return "servers"
}

