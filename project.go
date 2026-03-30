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
func (p *Project) Delete(ctx context.Context) error {
	rootFolder, err := p.GetRootFolder(ctx)
	if err != nil {
		log.Printf("Project %v's root folder not found: %v", p.Title, err)
	}

	err = rootFolder.DeleteEvenIfRoot(ctx)
	if err != nil {
		log.Printf("Could not delete project %v's root folder: %v", p.Title, err)
	}

	optimizations, err := p.GetOptimizations(ctx)
	if err != nil {
		log.Printf("Could not get optimizations of project %v to delete: %v", p.Title, err)
	}

	for _, o := range optimizations {
		o.Delete(ctx)
	}

	_, err = gorm.G[Project](gormDb).Where("id = ?", p.ID).Delete(ctx)

	return err
}

func (p *Project) GetOptimizations(ctx context.Context) ([]Optimization, error) {
	return gorm.G[Optimization](gormDb).Where("project_id = ?", p.ID).Find(ctx)
}
