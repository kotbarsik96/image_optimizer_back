package main

import (
	"context"
	_ "image/jpeg"
	_ "image/png"
	"log"
	"os"
	"path"
	"sync"
	"time"

	_ "golang.org/x/image/webp"
	"gorm.io/gorm"
)

type Folder struct {
	ID             uint      `gorm:"primarykey" json:"id"`
	ProjectID      *uint     `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"project_id,omitzero"`
	OptimizationID *uint     `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"optimization_id,omitzero"`
	Path           string    `json:"path,omitzero"`
	ParentID       *uint     `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"parent_id,omitzero"`
	Nested         []Folder  `gorm:"foreignKey:ParentID" json:"nested,omitzero"`
	Images         []Image   `json:"images,omitzero"`
	Storage        EStorage  `json:"storage"`
	CreatedAt      time.Time `json:"created_at,omitzero"`
	UpdatedAt      time.Time `json:"updated_at,omitzero"`
}

func (folder *Folder) GetNested(ctx context.Context) ([]Folder, error) {
	return gorm.G[Folder](gormDb).Where("parent_id = ?", folder.ID).Find(ctx)
}

// удалить папку, изображения и все вложенные папки и изображения. Не удаляет корневую папку (Path == ".")
func (folder *Folder) Delete(ctx context.Context) error {
	if folder.Path == "." {
		return ErrCannotDeleteRootFolder
	}

	return folder.DeleteEvenIfRoot(ctx)
}

// то же, что Folder.Delete, но может удалять и корневые папки (Path == ".")
func (folder *Folder) DeleteEvenIfRoot(ctx context.Context) error {
	nestedFolders, err := folder.GetNested(ctx)
	if err != nil {
		log.Printf("Could not get nested folders of %v to delete: %v", folder.Path, err)
	}

	for _, nestedFolder := range nestedFolders {
		err := nestedFolder.Delete(ctx)
		if err != nil {
			log.Printf("Could not delete nested folder of %v (%v): %v", folder.Path, nestedFolder.Path, err)
		}
	}

	images, err := gorm.G[Image](gormDb).Where("folder_id = ?", folder.ID).Find(ctx)
	if err != nil {
		return err
	}

	imageIds := []uint{}
	imagePaths := []string{}

	for _, img := range images {
		imageIds = append(imageIds, img.ID)
		imagePaths = append(imagePaths, path.Join(img.Path, img.OriginalFilename+"."+img.Extension))
	}

	storage := Storages[folder.Storage]
	err = storage.RemoveFiles(ctx, imagePaths)

	if err != nil {
		log.Printf("Error deleting folder images from %v storage: %v", folder.Storage, err)
	}

	_, err = gorm.G[Image](gormDb).
		Where("id IN ?", imageIds).
		Delete(ctx)

	if err != nil {
		log.Printf("Error deleting folder images from database: %v", err)
	}

	_, err = gorm.G[Folder](gormDb).
		Where("id = ?", folder.ID).
		Delete(ctx)

	if err != nil {
		log.Printf("Error deleting folder: %v", err)
	}

	return err
}

func (folder *Folder) OptimizeImages(ctx context.Context, sema chan int, opt Optimization, archiveDir, downloadsDir string, progress *Progress) {
	images, err := gorm.G[Image](gormDb).Where("folder_id = ?", folder.ID).Find(ctx)
	if err != nil {
		log.Printf("Could not get images of %v: %v", folder.Path, err)
	}

	var wg sync.WaitGroup
	for _, img := range images {
		wg.Add(1)
		go func(img Image) {
			defer wg.Done()

			sema <- 1
			defer func() { <-sema }()

			archiveImgDir := path.Join(archiveDir, folder.Path)
			err := os.MkdirAll(archiveImgDir, 0666)
			if err != nil {
				log.Printf("Could not make archive folder %v: %v", archiveImgDir, err)
				return
			}

			downloadImgDir := path.Join(downloadsDir, folder.Path)
			err = os.MkdirAll(downloadImgDir, 0666)
			if err != nil {
				log.Printf("Could not make download folder %v: %v", downloadImgDir, err)
				return
			}

			img.Optimize(ctx, opt, archiveImgDir, downloadImgDir, progress)
		}(img)
	}
	wg.Wait()

	nested, err := folder.GetNested(ctx)
	if err != nil {
		log.Printf("Could not get nested folders of %v: %v", folder.Path, err)
	}
	for _, nestedFolder := range nested {
		nestedFolder.OptimizeImages(ctx, sema, opt, archiveDir, downloadsDir, progress)
	}
}
