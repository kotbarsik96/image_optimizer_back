package main

import (
	"context"
	"fmt"
	"log"
	"mime/multipart"
	"net/url"
	"path"
	"slices"
	"strings"
	"time"

	"gorm.io/gorm"
)

type Image struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	FolderID  uint      `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"folder_id,omitzero"`
	Path      string    `json:"key,omitzero"` // путь в хранилище (s3, local). Может отличаться от пути папки Folder
	Extension string    `json:"extension,omitzero"`
	Filename  string    `json:"filename,omitzero"`
	Url       string    `gorm:"-" json:"url"`
	SizeBytes uint      `json:"size_bytes,omitzero"`
	Width     uint      `json:"width,omitzero"`
	Height    uint      `json:"height,omitzero"`
	Storage   EStorage  `json:"storage"`
	CreatedAt time.Time `json:"created_at,omitzero"`
	UpdatedAt time.Time `json:"updated_at,omitzero"`
}

func (image *Image) GetUrl() string {
	if image.Storage == EStorageLocal {
		return ""
	}

	storage := Storages[EStorageS3].(StorageS3)
	return storage.EndpointUrl + "/" + path.Join(storage.Bucket, storage.RootPath, url.PathEscape(image.Path))
}

func (image *Image) Delete(ctx context.Context) error {
	storage := Storages[image.Storage]

	err := storage.Remove(ctx, image.Path)
	if err != nil {
		return err
	}

	_, err = gorm.G[Image](gormDb).Where("id = ?", image.ID).Delete(ctx)
	return err
}

// Сформировать Image на основе переданного файла и пути
//
// imgPath - путь к изображению с названием файла и расширением (/path/to/img.png)
func NewImageFromFile(fileHeader *multipart.FileHeader, folder Folder, imgPath string) (*Image, error) {
	file, err := fileHeader.Open()
	if err != nil {
		return nil, err
	}
	defer file.Close()

	extension := strings.ToLower(path.Ext(fileHeader.Filename))[1:]
	filename := GetFilenameWithoutExtension(imgPath)

	img := Image{
		FolderID:  folder.ID,
		Path:      imgPath,
		Filename:  filename,
		Extension: extension,
		SizeBytes: uint(fileHeader.Size),
		Storage:   folder.Storage,
	}

	return &img, nil
}

func (image *Image) AfterFind(tx *gorm.DB) (err error) {
	image.Url = image.GetUrl()
	return
}

func (image *Image) AfterCreate(tx *gorm.DB) (err error) {
	image.Url = image.GetUrl()
	return
}

func (image *Image) Optimize(ctx context.Context, opt Optimization, archiveImgDir, downloadImgDir string, progress *Progress) {
	defer progress.Increment()

	sizes, _ := GetOptimizationSizes(opt.Sizes)
	extensions, _ := GetOptimizationExtensions(opt.Extensions)
	storage := Storages[image.Storage]

	if len(sizes) == 0 || len(extensions) == 0 {
		return
	}

	// названия форматов без расширений ({"image", "image-2x", "image-3x"})
	sizesFilenames := []string{}
	for i := range sizes {
		n := image.Filename
		if i > 0 {
			n = fmt.Sprintf("%v-%vx", image.Filename, i+1)
		}
		sizesFilenames = append(sizesFilenames, n)
	}

	// название оригинального изображения (с расширением)
	originalFileName := sizesFilenames[0] + "_original" + "." + image.Extension
	// полный путь к оригинальному изображению
	originalFilePath := path.Join(downloadImgDir, originalFileName)

	// скачивание изображения в путь originalFilePath
	_, err := storage.Download(ctx, image.Path, originalFilePath)
	if err != nil {
		log.Printf("Error while downloading image %v.%v for optimization %v: %v", image.Filename, image.Extension, opt.Title, err)
		return
	}

	// сначала пройтись по размерам и сделать ресайз на каждый размер
	for i, sizeFilename := range sizesFilenames {
		// путь к изображению, от которого будет происходить кодирование в заданные расширения [extensions]
		baseImgPath := originalFilePath

		// заресайзить изображение, если оно не оригинальное
		if sizes[i] != 100 {
			resizedFilename := sizeFilename + "." + image.Extension
			var resizedPath string
			if slices.Contains(extensions, image.Extension) {
				// если расширение оригинала есть в списке [extensions] - итоговое изображение поместить сразу в imageDir (archive)
				resizedPath = path.Join(archiveImgDir, resizedFilename)
			} else {
				// если расширение оригинала отсутствует в списке [extensions] - итоговое изображение поместить в [downloadsPath]
				resizedPath = path.Join(downloadImgDir, resizedFilename)
			}

			err := ResizeImage(
				originalFilePath,
				resizedPath,
				float64(sizes[i])/100)
			if err != nil {
				log.Printf("Error while resizing %v: %v", originalFilePath, err)
				continue
			}

			baseImgPath = resizedPath
		}

		// уже заресайженное изображение кодировать в заданные расширения [extensions]
		for _, ext := range extensions {
			filename := sizeFilename + "." + ext
			extPath := path.Join(archiveImgDir, filename)

			// изображение оригинального формата уже обработано, если оно не является оригиналом
			// оригинал же находится в папке downloads, поэтому его необходимо будет скопировать
			if ext == image.Extension && sizes[i] != 100 {
				continue
			}

			err := EncodeImageToExtension(baseImgPath, extPath)
			if err != nil {
				log.Printf("Error while encoding %v to %v: %v", baseImgPath, ext, err)
			}
		}
	}
}
