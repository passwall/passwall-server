package model

import (
	"time"
)

// Login ...
type Login struct {
	ID        uint       `gorm:"primary_key" json:"id"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at"`
	Title     string     `json:"title"`
	URL       string     `json:"url"`
	Username  string     `json:"username" encrypt:"true"`
	Password  string     `json:"password" encrypt:"true"`
}

type LoginDTO struct {
	ID       uint   `json:"id"`
	Title    string `json:"title"`
	URL      string `json:"url"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// ToLogin ...
func ToLogin(loginDTO *LoginDTO) *Login {
	return &Login{
		Title:    loginDTO.Title,
		URL:      loginDTO.URL,
		Username: loginDTO.Username,
		Password: loginDTO.Password,
	}
}

// ToLoginDTO ...
func ToLoginDTO(login *Login) *LoginDTO {
	return &LoginDTO{
		ID:       login.ID,
		Title:    login.Title,
		URL:      login.URL,
		Username: login.Username,
		Password: login.Password,
	}
}

// ToLoginDTOs ...
func ToLoginDTOs(logins []*Login) []*LoginDTO {
	loginDTOs := make([]*LoginDTO, len(logins))

	for i, itm := range logins {
		loginDTOs[i] = ToLoginDTO(itm)
	}

	return loginDTOs
}

// URLs ...
type URLs struct {
	Items []string `json:"urls"`
}

// AddItem ...
func (urls *URLs) AddItem(item string) {
	urls.Items = append(urls.Items, item)
}

// Password ...
type Password struct {
	Password string `json:"password"`
}

// You can send this data to API /posts/ endpoint with POST method to create dummy content
/*
{
	"Title":"Dummy Title",
	"URL":"http://dummywebsite.com",
	"Username": "dummyuser",
	"Password": "dummypassword"
}
*/
