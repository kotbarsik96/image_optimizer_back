package main

import (
	"context"
	"fmt"
	"image_optimizer/imgopt_s3"
	"net/http"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

type TUploadData struct {
	Err error  `json:"error"`
	Url string `json:"url"`
}

func RouteNewProject(c *gin.Context) {
	// получение текущего пользователя, создающего проект
	uploader := GetCurrentUploader(c)

	// создание нового проекта
	project := NewProjectEntity(uploader, c.GetString("title"))
	_, err := project.Save()
	if err != nil {
		fmt.Printf("Could not save project \"%v\": %v", project.Title, err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": "Could not save project",
		})
		return
	}

	// загрузка изображений на s3 и сохранение в базу данных
	form, _ := c.MultipartForm()
	images := form.File["image"]

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

	c.JSON(http.StatusOK, responseData)
}

func RouteGetProject(c *gin.Context) {

}
