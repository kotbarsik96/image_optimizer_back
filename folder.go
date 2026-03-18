package main

import (
	_ "image/jpeg"
	_ "image/png"
	"regexp"

	_ "golang.org/x/image/webp"
	"gorm.io/gorm"
)

var foldernameRegExp = regexp.MustCompile(`^[\pL\pM\pN._ -]+$`)

type Folder struct {
	gorm.Model
	ProjectID      uint `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	OptimizationID uint `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	Path           string
	Images         []Image
}

func (folder *Folder) GetNested() []FolderWithNested {
	return []FolderWithNested{}
	// nested := gorm.G[Folder](gormDb).
	// 	Where("")
}

type FolderWithNested struct {
	Folder
	Nested []FolderWithNested
}
