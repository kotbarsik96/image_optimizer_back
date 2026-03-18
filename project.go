package main

import (
	"context"
	"time"

	"gorm.io/gorm"
)

type Project struct {
	gorm.Model
	UploaderID uint `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	Folders    []Folder
	Title      string
}

func (project *Project) RootFolder() (Folder, error) {
	ctx := context.Background()
	return gorm.G[Folder](gormDb).
		Where("project_id = ? AND path = '.'", project.ID).
		First(ctx)
}

type ProjectPreview struct {
	ID         uint
	CreatedAt  time.Time
	UpdatedAt  time.Time
	RootFolder FolderWithNested
	Title      string
}
