package main

import (
	"context"
	"fmt"
	"image_optimizer/imgopt_s3"
	"mime/multipart"
	"path"
	"time"

	"gorm.io/gorm"
)

type Image struct {
	gorm.Model
	FolderID  uint `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	Url       string
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

		fullpath := path.Join(folder.Path, imgFileHeader.Filename)

		extension := path.Ext(imgFileHeader.Filename)[1:]
		url, _, err := bucket.UploadFile(context.TODO(), uploader.Uuid, fullpath, file, "image/"+extension)
		file.Close()

		if err == nil {
			img := Image{
				FolderID:  folder.ID,
				Extension: extension,
				Filename:  fmt.Sprintf(`%v_%v`, imgFileHeader.Filename, Md5(time.Now().String())),
				Url:       url,
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
