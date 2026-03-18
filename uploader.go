package main

import (
	"gorm.io/gorm"
)

type Uploader struct {
	gorm.Model
	Uuid     string
	Projects []Project
}
