package main

import (
	"fmt"
	"net/http"

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
		fmt.Printf("Could not save project \"%v\": %v\n", project.Title, err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": "Could not save project",
		})
		return
	}

	// загрузка изображений на s3 и сохранение в базу данных
	form, _ := c.MultipartForm()
	images := form.File["image"]
	responseData := UploadProjectImages(project, uploader, images)

	c.JSON(http.StatusOK, responseData)
}

func RouteGetProject(c *gin.Context) {

}
