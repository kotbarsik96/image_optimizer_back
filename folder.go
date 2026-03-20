package main

import (
	"context"
	_ "image/jpeg"
	_ "image/png"
	"log"
	"regexp"
	"time"

	_ "golang.org/x/image/webp"
	"gorm.io/gorm"
)

var foldernameRegExp = regexp.MustCompile(`^[\pL\pM\pN._ -]+$`)

type Folder struct {
	ID             uint      `gorm:"primarykey" json:"id"`
	ProjectID      *uint     `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"project_id,omitzero"`
	OptimizationID *uint     `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"optimization_id,omitzero"`
	Path           string    `json:"path,omitzero"`
	ParentID       *uint     `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"parent_id,omitzero"`
	Nested         []Folder  `gorm:"foreignKey:ParentID" json:"nested,omitzero"`
	Images         []Image   `json:"images,omitzero"`
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

	byBuckets := make(map[string][]string)
	for _, img := range images {
		imageIds = append(imageIds, img.ID)

		_, ok := byBuckets[img.Bucket]
		if !ok {
			byBuckets[img.Bucket] = []string{}
		}

		byBuckets[img.Bucket] = append(byBuckets[img.Bucket], img.Key)
	}

	for bucket, keys := range byBuckets {
		err := s3Actions.DeleteFiles(ctx, bucket, keys)
		if err != nil {
			log.Printf("Could not delete images from S3: %v", err)
		}
	}

	_, err = gorm.G[Image](gormDb).
		Where("id IN ?", imageIds).
		Delete(ctx)

	if err != nil {
		log.Printf("Error deleting folder images: %v", err)
	}

	_, err = gorm.G[Folder](gormDb).
		Where("id = ?", folder.ID).
		Delete(ctx)

	if err != nil {
		log.Printf("Error deleting folder: %v", err)
	}

	return err
}
