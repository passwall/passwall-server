package model

import (
	"time"
)

// Server ...
type Server struct {
	ID              uint       `gorm:"primary_key" json:"id"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	DeletedAt       *time.Time `json:"deleted_at"`
	Title           string     `json:"title"`
	IP              string     `json:"ip" encrypt:"true"`
	Username        string     `json:"username" encrypt:"true"`
	Password        string     `json:"password" encrypt:"true"`
	URL             string     `json:"url"`
	HostingUsername string     `json:"hosting_username" encrypt:"true"`
	HostingPassword string     `json:"hosting_password" encrypt:"true"`
	AdminUsername   string     `json:"admin_username" encrypt:"true"`
	AdminPassword   string     `json:"admin_password" encrypt:"true"`
	Extra           string     `json:"extra" encrypt:"true"`
}

// ServerDTO DTO object for Server type
type ServerDTO struct {
	ID              uint   `json:"id"`
	Title           string `json:"title"`
	IP              string `json:"ip"`
	Username        string `json:"username"`
	Password        string `json:"password"`
	URL             string `json:"url"`
	HostingUsername string `json:"hosting_username"`
	HostingPassword string `json:"hosting_password"`
	AdminUsername   string `json:"admin_username"`
	AdminPassword   string `json:"admin_password"`
	Extra           string `json:"extra"`
}

// ToServer ...
func ToServer(serverDTO *ServerDTO) *Server {
	return &Server{
		Title:           serverDTO.Title,
		IP:              serverDTO.IP,
		Username:        serverDTO.Username,
		Password:        serverDTO.Password,
		URL:             serverDTO.URL,
		HostingUsername: serverDTO.HostingUsername,
		HostingPassword: serverDTO.HostingPassword,
		AdminUsername:   serverDTO.AdminUsername,
		AdminPassword:   serverDTO.AdminPassword,
		Extra:           serverDTO.Extra,
	}
}

// ToServerDTO ...
func ToServerDTO(server *Server) *ServerDTO {
	return &ServerDTO{
		ID:              server.ID,
		Title:           server.Title,
		IP:              server.IP,
		Username:        server.Username,
		Password:        server.Password,
		URL:             server.URL,
		HostingUsername: server.HostingUsername,
		HostingPassword: server.HostingPassword,
		AdminUsername:   server.AdminUsername,
		AdminPassword:   server.AdminPassword,
		Extra:           server.Extra,
	}
}

// ToServerDTOs ...
func ToServerDTOs(servers []*Server) []*ServerDTO {
	serverDTOs := make([]*ServerDTO, len(servers))

	for i, itm := range servers {
		serverDTOs[i] = ToServerDTO(itm)
	}

	return serverDTOs
}

/* EXAMPLE JSON OBJECT
{
	"title":"Dummy Title",
	"ip":"192.168.1.1",
	"username": "dummyuser",
	"password": "dummypassword",
	"url":"http://dummywebsite.com",
	"hosting_username":"hostinguser",
	"hosting_password":"hostingpassword",
	"admin_username":"adminuser",
	"admin_password":"adminpassword"
}
*/
