package model

import (
	"encoding/base64"
	"reflect"
	"time"

	"github.com/passwall/passwall-server/internal/app"
	"github.com/spf13/viper"
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

func encryptField(loginDTO LoginDTO) LoginDTO {
	num := reflect.TypeOf(loginDTO).NumField()

	var tagVal string

	for i := 0; i < num; i++ {
		tagVal = reflect.TypeOf(loginDTO).Field(i).Tag.Get("encrypt")
		value := reflect.ValueOf(loginDTO).Field(i).String()

		if tagVal == "true" {
			value = base64.StdEncoding.EncodeToString(app.Encrypt(value, viper.GetString("server.passphrase")))
			reflect.ValueOf(&loginDTO).Elem().Field(i).SetString(value)
		}
	}

	return loginDTO
}

func decryptField(login Login) Login {
	num := reflect.TypeOf(login).NumField()

	var tagVal string

	for i := 0; i < num; i++ {
		tagVal = reflect.TypeOf(login).Field(i).Tag.Get("encrypt")
		value := reflect.ValueOf(login).Field(i).String()

		if tagVal == "true" {
			valueByte, _ := base64.StdEncoding.DecodeString(value)
			value = string(app.Decrypt(string(valueByte[:]), viper.GetString("server.passphrase")))
		}
	}

	return login
}

// ToLogin ...
func ToLogin(loginDTO *LoginDTO) *Login {

	*loginDTO = encryptField(*loginDTO)

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

	*login = decryptField(*login)

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
