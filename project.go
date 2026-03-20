package main

import (
	"context"
	"log"
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

// удалить проект и корневую папку. Удалит все связанные с проектом папки и изображения
func (project *Project) Delete() error {
	ctx := context.Background()

	rootFolder, err := project.RootFolder()
	if err != nil {
		log.Printf("Project %v's root folder not found: %v", project.Title, err)
	}

	err = rootFolder.DeleteEvenIfRoot()
	if err != nil {
		log.Printf("Could not delete project %v's root folder: %v", project.Title, err)
	}

	_, err = gorm.G[Project](gormDb).Where("id = ?", project.ID).Delete(ctx)

	return err
}

type ProjectPreview struct {
	ID         uint      `json:"id,omitzero"`
	CreatedAt  time.Time `json:"created_at,omitzero"`
	UpdatedAt  time.Time `json:"updated_at,omitzero"`
	RootFolder Folder    `json:"root_folder"`
	Title      string    `json:"title"`
}
