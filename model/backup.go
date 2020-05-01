package model

import "time"

// Backup Response
type Backup struct {
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type RestoreDTO struct {
	Name string `json:"name"`
}
