package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"mime/multipart"
	"net/url"
	"path"
	"slices"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
)

type Image struct {
	ID               uint      `gorm:"primarykey" json:"id"`
	FolderID         uint      `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"folder_id,omitzero"`
	StoragePath      string    `json:"key,omitzero"` // путь в хранилище (s3, local). Может отличаться от пути Folder.Path
	Extension        string    `json:"extension,omitzero"`
	Filename         string    `json:"filename,omitzero"`          // название, отображаемое пользователю, которое можно поменять
	OriginalFilename string    `json:"original_filename,omitzero"` // название для идентификации в хранилище. Задаётся при загрузке изображения и не меняется
	Url              string    `gorm:"-" json:"url"`
	SizeBytes        uint      `json:"size_bytes,omitzero"`
	Width            uint      `json:"width,omitzero"`
	Height           uint      `json:"height,omitzero"`
	Storage          EStorage  `json:"storage"`
	CreatedAt        time.Time `json:"created_at,omitzero"`
	UpdatedAt        time.Time `json:"updated_at,omitzero"`
}

func (i *Image) GetUrl() string {
	if i.Storage == EStorageLocal {
		return ""
	}

	storage := Storages[EStorageS3].(StorageS3)
	relPath := path.Join(url.PathEscape(i.StoragePath), url.PathEscape(i.OriginalFilename+"."+i.Extension))
	return storage.EndpointUrl + "/" + path.Join(storage.Bucket, storage.RootPath, relPath)
}

func (i *Image) Delete(ctx context.Context) error {
	storage := Storages[i.Storage]

	err := storage.Remove(ctx, path.Join(i.StoragePath, i.OriginalFilename+"."+i.Extension))
	if err != nil {
		return err
	}

	_, err = gorm.G[Image](gormDb).Where("id = ?", i.ID).Delete(ctx)
	return err
}

// Сформировать Image на основе переданного файла и пути
//
// dirPath - путь к папке изображения (/path/to)
func NewImageFromFile(fileHeader *multipart.FileHeader, folder Folder, dirPath string) (*Image, error) {
	file, err := fileHeader.Open()
	if err != nil {
		return nil, err
	}
	defer file.Close()

	extension, err := ToSupportedExtension(path.Ext(fileHeader.Filename)[1:])
	if err != nil {
		return nil, fmt.Errorf("%w: %v", err, extension)
	}

	nameWithoutExt := GetFilenameWithoutExtension(fileHeader.Filename)
	filename, err := CreateNotDuplicatedFilename(context.Background(), nameWithoutExt, extension, folder)
	if err != nil {
		return nil, err
	}

	img := Image{
		FolderID:         folder.ID,
		StoragePath:      dirPath,
		Filename:         filename,
		OriginalFilename: filename,
		Extension:        extension,
		SizeBytes:        uint(fileHeader.Size),
		Storage:          folder.Storage,
	}

	return &img, nil
}

// filename - без расширения
func CreateNotDuplicatedFilename(ctx context.Context, filename, extension string, folder Folder) (string, error) {
	_, err := gorm.G[Image](gormDb).Where("folder_id = ? AND filename = ? AND extension = ?", folder.ID, filename, extension).First(ctx)

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return filename, nil
	}

	if err != nil {
		return "", err
	}

	num := 2
	newName := filename + "-" + strconv.Itoa(num)
	for {
		dup, err := gorm.G[Image](gormDb).
			Where("folder_id = ? AND filename = ? AND extension = ?", folder.ID, newName, extension).
			Order("filename desc").
			First(ctx)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			break
		}

		if i := strings.LastIndexByte(dup.Filename, '-'); i >= 0 {
			dupNum, err := strconv.Atoi(dup.Filename[i+1:])
			if err == nil {
				newName = filename + "-" + strconv.Itoa(dupNum+1)
			} else {
				log.Printf("Failed to create not duplicated filename: %v", err)
			}
		}
	}

	return newName, nil
}

func (i *Image) AfterFind(tx *gorm.DB) (err error) {
	i.Url = i.GetUrl()
	return
}

func (i *Image) AfterCreate(tx *gorm.DB) (err error) {
	i.Url = i.GetUrl()
	return
}

func (image *Image) Optimize(ctx context.Context, opt Optimization, archiveImgDir, downloadImgDir string, progress *Progress) {
	defer func() {
		OptimizationsProgressStorage.Increment(progress)
	}()

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

	imageFullPath := path.Join(image.StoragePath, image.OriginalFilename+"."+image.Extension)

	// скачивание изображения в путь originalFilePath
	_, err := storage.Download(ctx, imageFullPath, originalFilePath)
	if err != nil {
		log.Printf("Error while downloading image %v.%v for optimization %v: %v", image.OriginalFilename, image.Extension, opt.Title, err)
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
