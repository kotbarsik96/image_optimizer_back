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

	details := TProgressDetails{}
	images := []*Image{}
	for _, fileHeader := range files {
		dirPath := path.Join(u.GetProjectsPath(), project.Title, folder.Path)
		img, err := NewImageFromFile(fileHeader, folder, dirPath)

		detail := TProgressDetailItem{
			Meta: map[string]any{"image": img},
		}
		if err != nil {
			detail.Error = err
		}
		details[img.Filename] = detail
		images = append(images, img)
	}

	totalCount := uint(len(details))
	progress := UploadsProgressStorage.NewProgress(&folder, u.ID, totalCount, details)

	fileStorage := Storages[folder.Storage]
	for i, img := range images {
		if details[img.Filename].Error != nil {
			UploadsProgressStorage.Increment(progress)
			continue
		}

		imgPath := path.Join(img.StoragePath, img.OriginalFilename+"."+img.Extension)

		err = fileStorage.PutImage(ctx, imgPath, files[i])
		if err == nil {
			err = gorm.G[Image](gormDb).Create(ctx, img)
		}

		UploadsProgressStorage.IncrementWithDetails(progress, img.Filename, TProgressDetailItem{Error: err})

		time.Sleep(2 * time.Second) //temp
	}

	return nil
}
