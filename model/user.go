package model

// User model should be something like this
type User struct {
	UUID           string `json:"uuid"`
	Email          string `json:"email"`
	MasterPassword string `json:"master_password"`
}
