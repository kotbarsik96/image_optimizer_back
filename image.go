package main

import (
	"bufio"
	"context"
	"fmt"
	"image"
	"image_optimizer/imgopt_s3"
	"mime/multipart"
	"path/filepath"
	"strings"
)

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

func GetImageEntity(id int) (TImageEntity, error) {
	entity := TImageEntity{}
	stmt := dbwrapper.DB.QueryRow("SELECT * FROM images WHERE id = ?", id)
	err := stmt.Scan(
		&entity.Id,
		&entity.Url,
		&entity.Extension,
		&entity.Filename,
		&entity.Path,
		&entity.Size_bytes,
		&entity.Width,
		&entity.Height,
		&entity.Created_at,
		&entity.Updated_at)
	return entity, err
}

func (img *TImageEntity) GetProject() (TProjectEntity, error) {
	var projectId int
	stmt := dbwrapper.DB.QueryRow("SELECT project_id FROM projects_images WHERE image_id = ?", img.Id)
	err := stmt.Scan(&projectId)
	if err != nil {
		fmt.Printf("Could not get project id related to img %v: %v\n", img.Id, err)
		return TProjectEntity{}, err
	}

	project, err := GetProjectEntity(projectId)
	if err != nil {
		fmt.Printf("Could not get project entity related to img %v: %v\n", img.Id, err)
		return project, err
	}

	return project, nil
}

func (img *TImageEntity) GetUploader() (TUploaderEntity, error) {
	project, err := img.GetProject()
	if err != nil {
		fmt.Printf("Could not get uploader entity related to img %v: %v\n", img.Id, err)
		return TUploaderEntity{}, err
	}

	return project.GetUploader()
}

func (img *TImageEntity) GetUrl() string {
	var bucket imgopt_s3.BucketBasis

	uploader, err := img.GetUploader()
	if err != nil {
		return ""
	}

	ipath := "/" + img.Path
	if ipath == "/." {
		ipath = ""
	}
	path := fmt.Sprintf("%v%v.%v", ipath, img.Filename, img.Extension)
	return bucket.GetFileUrl(uploader.Uuid, path)
}

func UploadProjectImages(project TProjectEntity, uploader TUploaderEntity, images []*multipart.FileHeader) map[string]TUploadData {
	var bucket imgopt_s3.BucketBasis

	responseData := make(map[string]TUploadData)

	for _, img := range images {
		data := TUploadData{}

		file, err := img.Open()
		if err != nil {
			data.Err = err
			continue
		}

		path := img.Filename

		extension := filepath.Ext(img.Filename)[1:]
		_, err = bucket.UploadFile(context.TODO(), uploader.Uuid, path, file, "image/"+extension)
		file.Close()

		data.Url = bucket.GetFileUrl(uploader.Uuid, path)

		if err == nil {
			dbimg, err := NewImageEntity(img, data.Url, path)
			err = dbimg.Save()
			if err != nil {
				data.Err = err
			}

			// присваивание изображения к проекту
			stmt, _ := dbwrapper.DB.Prepare("INSERT INTO projects_images VALUES(?, ?)")
			stmt.Exec(project.Id, dbimg.Id)
		}

		responseData[img.Filename] = data
	}

	return responseData
}
