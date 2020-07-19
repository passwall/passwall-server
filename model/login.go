package model

import (
	"time"
)

// Login ...
type Login struct {
	ID        uint       `gorm:"primary_key" json:"id" encrypt:"false"`
	CreatedAt time.Time  `json:"created_at" encrypt:"true"`
	UpdatedAt time.Time  `json:"updated_at" encrypt:"true"`
	DeletedAt *time.Time `json:"deleted_at" encrypt:"true"`
	Title     string     `json:"title" encrypt:"false"`
	URL       string     `json:"url" encrypt:"true"`
	Username  string     `json:"username" encrypt:"true"`
	Password  string     `json:"password" encrypt:"true"`
}

type LoginDTO struct {
	ID       uint   `json:"id" encrypt:"false"`
	Title    string `json:"title" encrypt:"false"`
	URL      string `json:"url" encrypt:"true"`
	Username string `json:"username" encrypt:"true"`
	Password string `json:"password" encrypt:"true"`
}

// ToLogin ...
func ToLogin(loginDTO *LoginDTO) *Login {

	//*loginDTO = app.EncryptLogin(*loginDTO)

	return &Login{
		Title:    loginDTO.Title,
		URL:      loginDTO.URL,
		Username: loginDTO.Username,
		Password: loginDTO.Password,
	}
}

// ToLoginDTO ...
func ToLoginDTO(login *Login) *LoginDTO {

	// trims := []string{"https://", "http://", "www."}
	// for i := range trims {
	// 	login.URL = strings.TrimPrefix(login.URL, trims[i])
	// }

	//*login = app.DecryptLogin(*login)

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
