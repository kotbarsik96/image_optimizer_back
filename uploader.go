package main

import (
	"path"
	"time"
)

type Uploader struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	Uuid      string    `json:"uuid,omitzero"`
	Projects  []Project `json:"projects,omitzero"`
	CreatedAt time.Time `json:"created_at,omitzero"`
	UpdatedAt time.Time `json:"updated_at,omitzero"`
}

func (u *Uploader) GetFolderPath() string {
	return path.Join("uploaders", u.Uuid)
}

func (u *Uploader) GetProjectsPath() string {
	return path.Join(u.GetFolderPath(), "projects")
}

func (u *Uploader) GetOptimizationsPath() string {
	return path.Join(u.GetFolderPath(), "optimizations")
}
