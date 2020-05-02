package model

// User model should be something like this
type User struct {
	UUID     string `json:"uuid" form:"-"`
	Username string `json:"Username" form:"Username"`
	Password string `json:"Password" form:"Password"`
}