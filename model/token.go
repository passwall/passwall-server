package model

import (
	"time"

	uuid "github.com/satori/go.uuid"
)

type Token struct {
	Id         int `gorm:"primary_key" json:"id"`
	UserId     int
	UUID       uuid.UUID `gorm:"type:uuid;type:varchar(100);"`
	Token      string    `gorm:"type:text;"`
	ExpiryTime time.Time
}
