package main

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"

	"github.com/gin-gonic/gin"
)

func RouteNewProject(c *gin.Context) {
	// получение текущего пользователя, создающего проект
	uploader := GetCurrentUploader(c)

	// создание нового проекта
	project := NewProjectEntity(uploader, c.PostForm("title"))
	_, err := project.Save()
	if err != nil {
		text := "Could not save project"
		utils.AbortWithError(
			c,
			http.StatusInternalServerError,
			text,
			fmt.Sprintf("%v \"%v\": %v", text, project.Title, err),
			nil)
		return
	}

	// создание папки ("." - значит в корне проекта)
	folder := NewFolderEntity(uploader.Id, ".")
	err = folder.Save()
	if err != nil {
		text := "Could not save folder"
		fullText := fmt.Sprintf("%v: %v", text, err)
		utils.AbortWithError(c,
			http.StatusInternalServerError,
			text,
			fullText,
			nil)
		return
	}

	// связать папку с проектом
	stmt, _ := dbwrapper.DB.Prepare("INSERT INTO projects_folders VALUES(?, ?)")
	stmt.Exec(project.Id, folder.Id)

	// загрузка изображений на s3 и сохранение в базу данных; изображение будет связано с папкой
	form, _ := c.MultipartForm()
	images := form.File["images"]
	fmt.Println(images)
	responseData := UploadProjectImages(uploader, folder, images)

	c.JSON(http.StatusCreated, responseData)
}

func RouteCreateFolder(c *gin.Context) {
	uploader := GetCurrentUploader(c)

	projectId, convErr := strconv.Atoi(c.Param("id"))
	project, err := GetProjectEntity(projectId)
	if err != nil || convErr != nil {
		text := "Project not found"
		textFull := fmt.Sprintf("%v: %v", text, err)
		utils.AbortWithError(
			c,
			http.StatusNotFound,
			text,
			textFull,
			nil,
		)
		return
	}

	if project.UploaderId != uploader.Id {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"error": "Unauthorized",
		})
		return
	}

	folderName := c.PostForm("name")
	if IsAcceptableFolderName(folderName) == false {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "Invalid folder name",
		})
		return
	}

	parentId, convErr := strconv.Atoi(c.PostForm("parent_id"))
	parentFolder, err := GetFolderEntity(parentId)
	if err != nil || convErr != nil {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{
			"error": "Parent folder not found",
		})
		return
	}

	newFolder := NewFolderEntity(uploader.Id, filepath.Join(parentFolder.Path, folderName))
	err = newFolder.Save()
	if err != nil {
		text := "Could not save folder"
		textFull := fmt.Sprintf("%v: %v", text, err)
		utils.AbortWithError(c,
			http.StatusInternalServerError,
			text,
			textFull,
			nil)
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"folder": newFolder,
	})
}

func RouteUploadFiles(c *gin.Context) {
	// получение текущего пользователя, загружающего изображения в проект
	uploader := GetCurrentUploader(c)

	folderId, err := strconv.Atoi(c.Param("id"))
	folder, fErr := GetFolderEntity(folderId)
	if err != nil || fErr != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "Folder not found",
		})
		return
	}

	if folder.UploaderId != uploader.Id {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"error": "Unauthorized",
		})
		return
	}

	form, _ := c.MultipartForm()
	images := form.File["images"]
	responseData := UploadProjectImages(uploader, folder, images)

	c.JSON(http.StatusCreated, responseData)
}

func RouteGetProject(c *gin.Context) {
	// uploader := GetCurrentUploader(c)

	// // получить id проекта из запроса
	// idParam := c.Param("id")
	// id, err := strconv.Atoi(idParam)
	// if err != nil {
	// 	c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
	// 		"error": fmt.Sprintf("Invalid parameter id in request: %v", idParam),
	// 	})
	// 	return
	// }

	// // получить проект
	// project, err := GetProjectEntity(id)
	// if err != nil {
	// 	text := "Project not found"
	// 	fulltext := fmt.Sprintf("%v: %v", text, err)
	// 	utils.AbortWithError(
	// 		c,
	// 		http.StatusNotFound,
	// 		text,
	// 		fulltext,
	// 		nil,
	// 	)
	// 	return
	// }

	// // пользователь может смотреть только свои проекты
	// if uploader.Id != project.UploaderId {
	// 	c.AbortWithStatusJSON(http.StatusNotFound, gin.H{
	// 		"error": "Project not found",
	// 	})
	// 	return
	// }

}
