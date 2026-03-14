package main

import (
	"bufio"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"mime/multipart"
	"path/filepath"
	"strings"

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

// конкретный файл изображения
type TImageEntity struct {
	TFile
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	Created_at string `json:"created_at"`
	Updated_at string `json:"updated_at"`
}

func NewImageEntity(imgFileheader *multipart.FileHeader, url, path string) (TImageEntity, error) {
	currentTime := utils.GetCurrentFormattedTime()

	extension := filepath.Ext(imgFileheader.Filename)[1:]

	file, err := imgFileheader.Open()

	imgConfig, _, _ := image.DecodeConfig(bufio.NewReader(file))

	fname := filepath.Base(path)
	fpath := filepath.Dir(path)

	entity := TImageEntity{
		TFile: TFile{
			Url:        url,
			Extension:  extension,
			Filename:   strings.Split(fname, ".")[0],
			Path:       fpath,
			Size_bytes: int(imgFileheader.Size),
		},
		Width:      imgConfig.Width,
		Height:     imgConfig.Height,
		Created_at: currentTime,
		Updated_at: currentTime,
	}

	return entity, err
}

func (img *TImageEntity) Save() error {
	id, err := dbwrapper.SaveEntity("images", img)
	img.Id = id
	return err
}
