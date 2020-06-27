package model

import (
	"time"
)

// Email ...
type Email struct {
	ID        uint       `gorm:"primary_key" json:"id"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at"`
	Title     string     `json:"title"`
	Email     string     `json:"email"`
	Password  string     `json:"password"`
}

// EmailDTO ...
type EmailDTO struct {
	ID       uint   `json:"id"`
	Title    string `json:"title"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

// ToEmail ...
func ToEmail(emailDTO *EmailDTO) *Email {
	return &Email{
		Title:    emailDTO.Title,
		Email:    emailDTO.Email,
		Password: emailDTO.Password,
	}
}

// ToEmailDTO ...
func ToEmailDTO(email *Email) *EmailDTO {
	return &EmailDTO{
		ID:       email.ID,
		Title:    email.Title,
		Email:    email.Email,
		Password: email.Password,
	}
}

// ToEmailDTOs ...
func ToEmailDTOs(emails []*Email) []*EmailDTO {
	emailDTOs := make([]*EmailDTO, len(emails))

	for i, itm := range emails {
		emailDTOs[i] = ToEmailDTO(itm)
	}

	return emailDTOs
}

/* EXAMPLE JSON OBJECT
{
	"title":"PassWall",
	"email":"hello@passwall.io",
	"password": "dummypassword"
}
*/
