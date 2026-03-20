package main

import (
	"context"
	"log"
	"time"

	"gorm.io/gorm"
)

type Project struct {
	ID            uint           `gorm:"primarykey" json:"id"`
	UploaderID    uint           `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"uploader_id,omitzero"`
	Folders       []Folder       `json:"folders,omitzero"`
	Optimizations []Optimization `json:"optimizations,omitzero"`
	Title         string         `json:"title,omitzero"`
	RootFolder    *Folder        `gorm:"-" json:"root_folder,omitzero"`
	CreatedAt     time.Time      `json:"created_at,omitzero"`
	UpdatedAt     time.Time      `json:"updated_at,omitzero"`
}

func (project *Project) GetRootFolder(ctx context.Context) (Folder, error) {
	return gorm.G[Folder](gormDb).
		Where("project_id = ? AND path = '.'", project.ID).
		Preload("Nested", nil).
		Preload("Images", nil).
		First(ctx)
}

// удалить проект и корневую папку. Удалит все связанные с проектом папки и изображения
func (project *Project) Delete(ctx context.Context) error {
	rootFolder, err := project.GetRootFolder(ctx)
	if err != nil {
		log.Printf("Project %v's root folder not found: %v", project.Title, err)
	}

	err = rootFolder.DeleteEvenIfRoot(ctx)
	if err != nil {
		log.Printf("Could not delete project %v's root folder: %v", project.Title, err)
	}

	_, err = gorm.G[Project](gormDb).Where("id = ?", project.ID).Delete(ctx)

	return err
}

func (project *Project) GetOptimizations(ctx context.Context) ([]Optimization, error) {
	return gorm.G[Optimization](gormDb).Where("project_id = ?", project.ID).Find(ctx)
}

type ProjectPreview struct {
	ID         uint      `json:"id,omitzero"`
	CreatedAt  time.Time `json:"created_at,omitzero"`
	UpdatedAt  time.Time `json:"updated_at,omitzero"`
	RootFolder Folder    `json:"root_folder"`
	Title      string    `json:"title"`
}
