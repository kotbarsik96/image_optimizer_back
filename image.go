package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"mime/multipart"
	"net/url"
	"os"
	"path"
	"slices"
	"strings"
	"time"

	"gorm.io/gorm"
)

type Image struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	FolderID  uint      `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"folder_id,omitzero"`
	S3Url     string    `json:"s3_url,omitzero"`
	Bucket    string    `json:"bucket,omitzero"`
	Key       string    `json:"key,omitzero"`
	Extension string    `json:"extension,omitzero"`
	Filename  string    `json:"filename,omitzero"`
	Url       string    `gorm:"-" json:"url"`
	SizeBytes uint      `json:"size_bytes,omitzero"`
	Width     uint      `json:"width,omitzero"`
	Height    uint      `json:"height,omitzero"`
	CreatedAt time.Time `json:"created_at,omitzero"`
	UpdatedAt time.Time `json:"updated_at,omitzero"`
}

func (image *Image) GetUrl() string {
	return (fmt.Sprintf("%v/%v", image.S3Url, url.PathEscape(path.Join(image.Bucket, image.Key))))
}

func (image *Image) Delete(ctx context.Context) error {
	err := s3Actions.DeleteFiles(ctx, image.Bucket, []string{image.Key})
	if err != nil {
		log.Printf("Could not delete images from S3: %v", err)
	}

	_, err = gorm.G[Image](gormDb).Where("id = ?", image.ID).Delete(ctx)
	return err
}

type UploadData struct {
	Err   string `json:"error"`
	Image Image  `json:"image"`
}

func UploadProjectImages(
	ctx context.Context,
	uploader Uploader,
	folder Folder,
	images []*multipart.FileHeader,
) []UploadData {
	responseData := []UploadData{}

	for _, imgFileHeader := range images {
		data := UploadData{}

		file, err := imgFileHeader.Open()
		if err != nil {
			data.Err = ErrorByCurrentEnv("Could not open image file", err).Error()
			continue
		}

		extension := strings.ToLower(path.Ext(imgFileHeader.Filename))[1:]

		s3Url := os.Getenv("S3_ENDPOINT_URL")
		s3Bucket := os.Getenv("S3_BUCKET")

		filename := GetFilenameWithoutExtension(imgFileHeader.Filename)

		hash := Md5(time.Now().String())
		filenameHashed := fmt.Sprintf(`%v_%v`, filename, hash)
		key := path.Join(os.Getenv("PROJECT_NAME"), uploader.Uuid, "projects")
		if folder.Path != "." {
			key = path.Join(key, folder.Path)
		}
		key = path.Join(key, fmt.Sprintf("%v.%v", filenameHashed, extension))

		_, err = s3Actions.UploadFile(ctx, s3Bucket, key, file, "image/"+extension)

		file.Close()

		// если изображение с таким названием уже существует в данной папке - добавить в название хэш
		_, existsErr := gorm.G[Image](gormDb).
			Where("folder_id = ? AND filename = ? AND extension = ?", folder.ID, filename, extension).
			First(ctx)
		if existsErr == nil || !errors.Is(existsErr, gorm.ErrRecordNotFound) {
			filename = filenameHashed
		}

		if err == nil {
			img := Image{
				FolderID:  folder.ID,
				S3Url:     s3Url,
				Bucket:    s3Bucket,
				Key:       key,
				Extension: extension,
				Filename:  filename,
				SizeBytes: uint(imgFileHeader.Size),
			}
			err := gorm.G[Image](gormDb).Create(ctx, &img)

			if err != nil {
				data.Err = ErrorByCurrentEnv("Could not save image", err).Error()
			} else {
				data.Image = img
			}
		} else {
			data.Err = ErrorByCurrentEnv("Could not upload image", err).Error()
		}

		responseData = append(responseData, data)
	}

	return responseData
}

func (image *Image) AfterFind(tx *gorm.DB) (err error) {
	image.Url = image.GetUrl()
	return
}

func (image *Image) AfterCreate(tx *gorm.DB) (err error) {
	image.Url = image.GetUrl()
	return
}

func (image *Image) Optimize(ctx context.Context, opt Optimization, outputDir string) {
	sizes, _ := GetOptimizationSizes(opt.Sizes)
	extensions, _ := GetOptimizationExtensions(opt.Extensions)

	// названия форматов без расширений ({"image", "image-2x", "image-3x"})
	sizesFilenames := []string{}
	for i := range sizes {
		n := image.Filename
		if i > 0 {
			n = fmt.Sprintf("%v-%vx", image.Filename, i+1)
		}
		sizesFilenames = append(sizesFilenames, n)
	}

	// название оригинального скачанного с s3 изображения (с расширением)
	originalFileName := fmt.Sprintf("%v.%v", sizesFilenames[len(sizesFilenames)-1], image.Extension)
	// полный путь к оригинальному изображению
	originalFilePath := path.Join(outputDir, originalFileName)

	// скачивание изображения в путь originalFilePath
	_, err := s3Actions.DownloadFile(ctx, image.Bucket, image.Key, originalFilePath)
	if err != nil {
		log.Printf("Error while downloading image %v.%v for optimization %v: %v", image.Filename, image.Extension, opt.Title, err)
		return
	}

	// сначала пройтись по размерам и сделать ресайз на каждый размер
	for i, sizeFilename := range sizesFilenames {
		// путь к изображению, от которого будет происходить кодирование в заданные расширения [extensions]
		baseImgPath := originalFilePath

		// заресайзить изображение, если оно оригинальное
		if i < len(sizesFilenames)-1 {
			resizedPath := path.Join(outputDir, fmt.Sprintf("%v.%v", sizeFilename, image.Extension))
			err := ResizeImage(
				originalFilePath,
				resizedPath,
				float64(sizes[i])/100)
			if err != nil {
				log.Printf("Error while resizing %v.%v: %v", image.Filename, image.Extension, err)
				continue
			}

			baseImgPath = resizedPath
		}

		// уже заресайженное изображение кодировать в заданные расширения [extensions]
		for _, ext := range extensions {
			filename := fmt.Sprintf("%v.%v", sizeFilename, ext)
			extPath := path.Join(outputDir, filename)
			err := EncodeImageToExtension(baseImgPath, extPath)
			if err != nil {
				log.Printf("Error while encoding %v to %v: %v", baseImgPath, ext, err)
			}
		}

		// удалить текущее изображение, если его формат не указан в [extensions]
		originalExt := path.Ext(baseImgPath)[1:]
		if !slices.Contains(extensions, originalExt) {
			err := os.Remove(baseImgPath)
			if err != nil {
				log.Printf("Could not remove file %v: %v", baseImgPath, err)
			}
		}
	}
}
