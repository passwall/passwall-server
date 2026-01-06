package model

import "time"

// Backup Response
type Backup struct {
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

// RestoreDTO file name for restore
type RestoreDTO struct {
	Name string `json:"name"`
}
