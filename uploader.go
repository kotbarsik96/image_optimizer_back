package main

import (
	"context"
	"mime/multipart"
	"path"
	"time"

	"gorm.io/gorm"
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

func (u *Uploader) GetDownloadsDir() string {
	return path.Join(u.GetFolderPath(), "downloads")
}

func (u *Uploader) GetOptimizationsPath() string {
	return path.Join(u.GetFolderPath(), "optimizations")
}

func (u *Uploader) UploadFiles(folder Folder, files []*multipart.FileHeader) error {
	ctx := context.Background()

	project, err := gorm.G[Project](gormDb).Where("id = ?", folder.ProjectID).First(ctx)
	if err != nil {
		return err
	}

	storage := Storages[folder.Storage]

	for _, fileHeader := range files {
		dirPath := path.Join(u.GetProjectsPath(), project.Title, folder.Path)

		img, err := NewImageFromFile(fileHeader, folder, dirPath)
		if err != nil {
			continue
		}

		imgPath := path.Join(dirPath, img.OriginalFilename+"."+img.Extension)

		err = storage.PutImage(ctx, imgPath, fileHeader)
		if err != nil {
			continue
		}

		err = gorm.G[Image](gormDb).Create(ctx, img)
	}

	return nil
}
