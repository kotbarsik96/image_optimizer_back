package main

import (
	"context"
	"time"

	"gorm.io/gorm"
)

type Project struct {
	ID         uint      `gorm:"primarykey" json:"id"`
	UploaderID uint      `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"uploader_id,omitzero"`
	Folders    []Folder  `json:"folders,omitzero"`
	Title      string    `json:"title,omitzero"`
	CreatedAt  time.Time `json:"created_at,omitzero"`
	UpdatedAt  time.Time `json:"updated_at,omitzero"`
}

func (project *Project) RootFolder() (Folder, error) {
	ctx := context.Background()
	return gorm.G[Folder](gormDb).
		Where("project_id = ? AND path = '.'", project.ID).
		Preload("Nested", nil).
		First(ctx)
}

type ProjectPreview struct {
	ID         uint      `json:"id,omitzero"`
	CreatedAt  time.Time `json:"created_at,omitzero"`
	UpdatedAt  time.Time `json:"updated_at,omitzero"`
	RootFolder Folder    `json:"root_folder"`
	Title      string    `json:"title"`
}
