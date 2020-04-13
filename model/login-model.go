package model

import "time"

type Login struct {
	ID        uint `gorm:"primary_key"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
	URL       string
	Username  string
	Password  string
}

type Result struct {
	Status  string
	Message string
}

// You can send this data to API /posts/ endpoint with POST method to create dummy content
/*
{
	"URL":"http://dummywebsite.com",
	"Username": "dummyuser",
	"Password": "dummypassword"
}
*/
