package main

import "time"

type Uploader struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	Uuid      string    `json:"uuid,omitzero"`
	Projects  []Project `json:"projects,omitzero"`
	CreatedAt time.Time `json:"created_at,omitzero"`
	UpdatedAt time.Time `json:"updated_at,omitzero"`
}
