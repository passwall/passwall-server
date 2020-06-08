package model

import (
	"time"

	uuid "github.com/satori/go.uuid"
)

//AuthLoginDTO ...
type AuthLoginDTO struct {
	Email          string `validate:"required" json:"email"`
	MasterPassword string `validate:"required" json:"master_password"`
}

//AuthLoginResponse ...
type AuthLoginResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	SecureKey    string `json:"secure_key"`
	UserDTOTable
}

//TokenDetailsDTO ...
type TokenDetailsDTO struct {
	AccessToken   string `json:"access_token"`
	RefreshToken  string `json:"refresh_token"`
	AtExpiresTime time.Time
	RtExpiresTime time.Time
	AtUUID        uuid.UUID
	RtUUID        uuid.UUID
	SecureKey     string `json:"secure_key"`
}
