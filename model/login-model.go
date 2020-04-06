package model

import (
	"github.com/jinzhu/gorm"
)

type Login struct {
	gorm.Model
	URL      string
	Username string
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
