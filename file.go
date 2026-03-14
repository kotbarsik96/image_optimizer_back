package main

import (
	_ "image/jpeg"
	_ "image/png"

	_ "golang.org/x/image/webp"
)

// папка: формируется из списка файлов []TFile
type TFolder struct {
	// название папки
	Name string
	// подпапки
	Folders []TFolder
	// файлы внутри папки
	Files []TFile
}

// файл с общей информацией
type TFile struct {
	Id         int    `json:"id"`
	Url        string `json:"url"`
	Extension  string `json:"extension"`
	Filename   string `json:"filename"`
	Path       string `json:"path"`
	Size_bytes int    `json:"size_bytes"`
}
