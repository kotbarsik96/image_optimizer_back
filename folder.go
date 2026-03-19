package main

import (
	"context"
	_ "image/jpeg"
	_ "image/png"
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

type FolderWithNested struct {
	Folder
	Nested []Folder `json:"nested"`
}
