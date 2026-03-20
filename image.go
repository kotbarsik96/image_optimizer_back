package main

import (
	"context"
	"fmt"
	"log"
	"mime/multipart"
	"net/url"
	"os"
	"path"
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

func (image *Image) Delete() error {
	ctx := context.Background()

	err := s3Actions.DeleteFiles(ctx, image.Bucket, []string{image.Key})
	if err != nil {
		log.Printf("Could not delete images from S3: %v", err)
	}

	_, err = gorm.G[Image](gormDb).Where("id = ?", image.ID).Delete(ctx)
	return err
}

type UploadData struct {
	Err   error `json:"error"`
	Image Image `json:"image"`
}

func UploadProjectImages(
	uploader Uploader,
	folder Folder,
	images []*multipart.FileHeader,
) []UploadData {
	responseData := []UploadData{}

	for _, imgFileHeader := range images {
		ctx := context.Background()

		data := UploadData{}

		file, err := imgFileHeader.Open()
		if err != nil {
			data.Err = err
			continue
		}

		extension := path.Ext(imgFileHeader.Filename)[1:]

		s3Url := os.Getenv("S3_ENDPOINT_URL")
		s3Bucket := os.Getenv("S3_BUCKET")

		filename := GetFilenameWithoutExtension(imgFileHeader.Filename)

		hash := Md5(time.Now().String())
		filenameHashed := fmt.Sprintf(`%v_%v`, filename, hash)
		key := path.Join(os.Getenv("PROJECT_NAME"), uploader.Uuid)
		if folder.Path != "." {
			key = path.Join(key, folder.Path)
		}
		key = path.Join(key, fmt.Sprintf("%v.%v", filenameHashed, extension))

		_, err = s3Actions.UploadFile(context.Background(), s3Bucket, key, file, "image/"+extension)

		file.Close()

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
				data.Err = err
			} else {
				data.Image = img
			}
		} else {
			data.Err = err
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
