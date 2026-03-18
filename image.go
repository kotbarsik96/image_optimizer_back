package main

import (
	"bufio"
	"context"
	"fmt"
	"image"
	"image_optimizer/imgopt_db"
	"image_optimizer/imgopt_s3"
	"mime/multipart"
	"path"
	"strings"
)

type TImageEntity struct {
	TFileEntity
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

func NewImageEntity(imgFileheader *multipart.FileHeader, url string, folder TFolderEntity, filename string) (TImageEntity, error) {
	currentTime := utils.GetCurrentFormattedTime()

	extension := path.Ext(imgFileheader.Filename)[1:]

	file, err := imgFileheader.Open()

	imgConfig, _, _ := image.DecodeConfig(bufio.NewReader(file))

	entity := TImageEntity{
		TFileEntity: TFileEntity{
			Url:       url,
			FolderId:  folder.Id,
			Extension: extension,
			Filename:  strings.Split(filename, ".")[0],
			SizeBytes: int(imgFileheader.Size),
		},
		Width:     imgConfig.Width,
		Height:    imgConfig.Height,
		CreatedAt: currentTime,
		UpdatedAt: currentTime,
	}

	file.Close()

	return entity, err
}

func (img *TImageEntity) ScanFullRow(row imgopt_db.DatabaseRow) error {
	return row.Scan(&img.Id,
		&img.FolderId,
		&img.Url,
		&img.Extension,
		&img.Filename,
		&img.SizeBytes,
		&img.Width,
		&img.Height,
		&img.CreatedAt,
		&img.UpdatedAt)
}

func (img *TImageEntity) Save() error {
	id, err := dbwrapper.SaveEntity("images", img)
	img.Id = id
	return err
}

func GetImageEntity(id int) (TImageEntity, error) {
	entity := TImageEntity{}
	row := dbwrapper.DB.QueryRow("SELECT * FROM images WHERE id = ?", id)
	err := entity.ScanFullRow(row)
	return entity, err
}

func (img *TImageEntity) GetFolder() (TFolderEntity, error) {
	folder := TFolderEntity{}

	row := dbwrapper.DB.QueryRow("SELECT * FROM folders WHERE id = ?", img.FolderId)
	err := folder.ScanFullRow(row)

	return folder, err
}

// func (img *TImageEntity) GetUrl() string {
// 	var bucket imgopt_s3.BucketBasis

// 	var uploaderUuid string
// 	var folderPath string
// 	err := dbwrapper.DB.QueryRow(`
// 		SELECT
// 			uploaders.uuid AS uploader_uuid,
// 			folders.path AS folder_path
// 		FROM uploaders
// 		JOIN projects ON projects.uploader_id = uploaders.id
// 		JOIN folders ON folders.project_id = projects.id
// 		JOIN images ON images.folder_id = folders.id
// 		WHERE images.id = ?
// 	`, img.Id).
// 		Scan(&uploaderUuid, &folderPath)
// 	if err != nil {
// 		return ""
// 	}

// 	ipath := folderPath + "/"
// 	if ipath == "./" {
// 		ipath = ""
// 	}
// 	path := fmt.Sprintf("%v%v.%v", ipath, img.Filename, img.Extension)
// 	return bucket.GetFileUrl(uploaderUuid, path)
// }

type TUploadData struct {
	Err   error        `json:"error"`
	Image TImageEntity `json:"image"`
}

func UploadProjectImages(
	uploader TUploaderEntity,
	folder TFolderEntity,
	images []*multipart.FileHeader,
) []TUploadData {
	var bucket imgopt_s3.BucketBasis

	responseData := []TUploadData{}

	for _, img := range images {
		data := TUploadData{}

		file, err := img.Open()
		if err != nil {
			data.Err = err
			continue
		}

		fullpath := path.Join(folder.Path, img.Filename)

		extension := path.Ext(img.Filename)[1:]
		url, _, err := bucket.UploadFile(context.TODO(), uploader.Uuid, fullpath, file, "image/"+extension)
		file.Close()

		if err == nil {
			imgEntity, err := NewImageEntity(img, url, folder, img.Filename)
			err = imgEntity.Save()
			fmt.Println(err)
			if err != nil {
				data.Err = err
			} else {
				data.Image = imgEntity
			}
		} else {
			data.Err = err
		}

		responseData = append(responseData, data)
	}

	return responseData
}
