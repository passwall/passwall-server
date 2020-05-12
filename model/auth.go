package model

import (
	"time"

	uuid "github.com/satori/go.uuid"
)

//AuthLoginDTO ...
type AuthLoginDTO struct {
	Username string `validate:"required" json:"username"`
	Password string `validate:"required" json:"password"`
}

//TokenDetailsDTO ...
type TokenDetailsDTO struct {
	AccessToken   string `json:"access_token"`
	RefreshToken  string `json:"refresh_token"`
	AtExpiresTime time.Time
	RtExpiresTime time.Time
	AtUUID        uuid.UUID
	RtUUID        uuid.UUID
}
