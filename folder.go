package main

import (
	"context"
	"fmt"
	_ "image/jpeg"
	_ "image/png"
	"log"
	"path"
	"regexp"
	"time"

	_ "golang.org/x/image/webp"
	"gorm.io/gorm"
)

var foldernameRegExp = regexp.MustCompile(`^[\pL\pM\pN._ -]+$`)

type Folder struct {
	ID             uint      `gorm:"primarykey" json:"id"`
	ProjectID      uint      `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"project_id,omitzero"`
	OptimizationID uint      `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"optimization_id,omitzero"`
	Path           string    `json:"path,omitzero"`
	Nested         []Folder  `gorm:"-" json:"nested,omitzero"`
	Images         []Image   `json:"images,omitzero"`
	CreatedAt      time.Time `json:"created_at,omitzero"`
	UpdatedAt      time.Time `json:"updated_at,omitzero"`
}

func (folder *Folder) GetNested() []Folder {
	ctx := context.Background()

	folders, _ := gorm.G[Folder](gormDb).
		Select("id", "path", "created_at", "updated_at").
		Where("project_id = ? AND id != ?", folder.ProjectID, folder.ID).
		Find(ctx)

	folders = FilterSlice(folders, func(index int, item Folder, slice []Folder) bool {
		return folder.Path == path.Dir(item.Path)
	})

	return folders
}

func (folder *Folder) Delete() error {
	ctx := context.Background()

	if folder.Path == "." {
		return fmt.Errorf("Cannot delete root folder")
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
