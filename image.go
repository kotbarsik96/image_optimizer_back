package main

import (
	"context"
	"fmt"
	"image_optimizer/imgopt_s3"
	"mime/multipart"
	"os"
	"path"
	"time"

	"gorm.io/gorm"
)

type Image struct {
	gorm.Model
	FolderID  uint `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	S3Url     string
	Bucket    string
	Key       string
	Extension string
	Filename  string
	SizeBytes string
	Width     uint
	Height    uint
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
	var bucket imgopt_s3.BucketBasis

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
		key := fmt.Sprintf("%v/%v/%v", os.Getenv("PROJECT_NAME"), uploader.Uuid, folder.Path)

		_, err = bucket.UploadFile(context.Background(), s3Bucket, key, file, "image/"+extension)

		file.Close()

		if err == nil {
			img := Image{
				FolderID:  folder.ID,
				S3Url:     s3Url,
				Bucket:    s3Bucket,
				Key:       key,
				Extension: extension,
				Filename:  fmt.Sprintf(`%v_%v`, imgFileHeader.Filename, Md5(time.Now().String())),
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
