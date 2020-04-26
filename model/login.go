package model

import (
	"strings"
	"time"
)

// Login ...
type Login struct {
	ID        uint `gorm:"primary_key"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
	URL       string
	Username  string
	Password  string
}

type LoginDTO struct {
	ID       uint
	URL      string
	Username string
	Password string
}

// ToLogin ...
func ToLogin(loginDTO LoginDTO) Login {
	return Login{
		URL:      loginDTO.URL,
		Username: loginDTO.Username,
		Password: loginDTO.Password,
	}
}

// ToLoginDTO ...
func ToLoginDTO(login Login) LoginDTO {

	trims := []string{"https://", "http://", "www."}
	for i := range trims {
		login.URL = strings.TrimPrefix(login.URL, trims[i])
	}

	return LoginDTO{
		ID:       login.ID,
		URL:      login.URL,
		Username: login.Username,
		Password: login.Password,
	}
}

// ToLoginDTOs ...
func ToLoginDTOs(logins []Login) []LoginDTO {
	loginDTOs := make([]LoginDTO, len(logins))

	for i, itm := range logins {
		loginDTOs[i] = ToLoginDTO(itm)
	}

	return loginDTOs
}

// URLs ...
type URLs struct {
	Items []string `json:"URLs"`
}

// AddItem ...
func (urls *URLs) AddItem(item string) {
	urls.Items = append(urls.Items, item)
}

// Password ...
type Password struct {
	Password string
}

// You can send this data to API /posts/ endpoint with POST method to create dummy content
/*
{
	"URL":"http://dummywebsite.com",
	"Username": "dummyuser",
	"Password": "dummypassword"
}
*/
