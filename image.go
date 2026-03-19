package main

import (
	"context"
	"fmt"
	"image_optimizer/imgopt_s3"
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
	SizeBytes uint      `json:"size_bytes,omitzero"`
	Width     uint      `json:"width,omitzero"`
	Height    uint      `json:"height,omitzero"`
	CreatedAt time.Time `json:"created_at,omitzero"`
	UpdatedAt time.Time `json:"updated_at,omitzero"`
}

func (image *Image) GetUrl() string {
	return (fmt.Sprintf("%v/%v", image.S3Url, url.PathEscape(path.Join(image.Bucket, image.Key))))
}

type ImageWithUrl struct {
	Image
	Url string
}

type UploadData struct {
	Err   error        `json:"error"`
	Image ImageWithUrl `json:"image"`
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

		hash := Md5(time.Now().String())
		filename := fmt.Sprintf(`%v_%v`, GetFilenameWithoutExtension(imgFileHeader.Filename), hash)
		key := path.Join(os.Getenv("PROJECT_NAME"), uploader.Uuid)
		if folder.Path != "." {
			key = path.Join(key, folder.Path)
		}
		key = path.Join(key, fmt.Sprintf("%v.%v", filename, extension))

		_, err = bucket.UploadFile(context.Background(), s3Bucket, key, file, "image/"+extension)

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
				data.Image = ImageWithUrl{
					Image: img,
					Url:   img.GetUrl(),
				}
			}
		} else {
			data.Err = err
		}

		responseData = append(responseData, data)
	}

	return responseData
}
