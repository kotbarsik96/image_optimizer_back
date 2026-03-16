package main

import (
	"fmt"
	"net/http"
	"path"
	"strconv"

	"github.com/gin-gonic/gin"
)

func RouteGetProjectsList(c *gin.Context) {
	uploader := GetCurrentUploader(c)

	projects := []TProjectPreview{}

	rows, err := dbwrapper.DB.Query(`
		SELECT id, title, created_at, updated_at 
		FROM projects
		WHERE uploader_id = ?
	`, uploader.Id)
	if err != nil {
		utils.AbortWithError(c, http.StatusInternalServerError, "Could not get projects", err)
		return
	}

	for rows.Next() {
		project := TProjectPreview{
			Optimizations: []TOptimizationPreview{},
		}
		err := rows.Scan(&project.Id, &project.Title, &project.CreatedAt, &project.UpdatedAt)
		if err != nil {
			utils.AbortWithError(c, http.StatusInternalServerError, "Error while trying to get project", err)
			return
		}

		oRows, err := dbwrapper.DB.Query(`
			SELECT id, output_extension, created_at, updated_at
			FROM optimizations
			WHERE project_id = ?
		`, project.Id)
		if err != nil {
			utils.AbortWithError(c, http.StatusInternalServerError, fmt.Sprintf("Could not get optimizations for project %v", project.Id), err)
			return
		}

		for oRows.Next() {
			opt := TOptimizationPreview{}
			err := oRows.Scan(&opt.Id, &opt.OutputExtension, &opt.CreatedAt, &opt.UpdatedAt)
			if err != nil {
				utils.AbortWithError(
					c,
					http.StatusInternalServerError,
					fmt.Sprintf("Error while trying to get optimization for project %v", project.Id),
					err,
				)
				return
			}

			project.Optimizations = append(project.Optimizations, opt)
		}

		projects = append(projects, project)
	}

	c.JSON(http.StatusOK, gin.H{
		"data": projects,
	})
}

func RouteGetProject(c *gin.Context) {
	uploader := GetCurrentUploader(c)

	idParam := c.Param("id")
	id, convErr := strconv.Atoi(idParam)
	project, err := GetProjectEntity(id)
	if err != nil || convErr != nil {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{
			"error": "Project not found",
		})
		return
	}

	if project.UploaderId != uploader.Id {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"error": "Unauthorized",
		})
		return
	}

	project, err = GetProjectEntity(1)

	tree, err := project.GetFoldersTree()
	if err != nil {
		utils.AbortWithError(c, http.StatusInternalServerError, "Could not get project", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": tree,
	})
}

func RouteNewProject(c *gin.Context) {
	// получение текущего пользователя, создающего проект
	uploader := GetCurrentUploader(c)

	// создание нового проекта
	project := NewProjectEntity(uploader, c.PostForm("title"))
	_, err := project.Save()
	if err != nil {
		utils.AbortWithError(c, http.StatusInternalServerError, "Could not save project", err)
		return
	}

	// создание папки ("." - значит в корне проекта)
	folder := NewFolderEntity(uploader.Id, ".")
	err = folder.Save()
	if err != nil {
		utils.AbortWithError(c, http.StatusInternalServerError, "Could not save folder", err)
		return
	}

	// связать папку с проектом
	folder.SaveToProject(project.Id)

	// загрузка изображений на s3 и сохранение в базу данных; изображение будет связано с папкой
	form, _ := c.MultipartForm()
	images := form.File["images"]
	fmt.Println(images)
	responseData := UploadProjectImages(uploader, folder, images)

	c.JSON(http.StatusCreated, responseData)
}

func RouteNewFolder(c *gin.Context) {
	uploader := GetCurrentUploader(c)

	projectId, convErr := strconv.Atoi(c.Param("id"))
	project, err := GetProjectEntity(projectId)
	if err != nil || convErr != nil {
		utils.AbortWithError(c, http.StatusNotFound, "Project not found", err)
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

	newFolderPath := path.Join(parentFolder.Path, folderName)

	existingFolder := TFolderEntity{}
	row := dbwrapper.DB.QueryRow("SELECT * FROM folders WHERE path = ? AND uploader_id = ?", newFolderPath, uploader.Id)
	err = existingFolder.ScanFullRow(row)
	if err == nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "Folder already exists",
		})
		return
	}

	newFolder := NewFolderEntity(uploader.Id, newFolderPath)
	err = newFolder.Save()
	if err != nil {
		utils.AbortWithError(c, http.StatusInternalServerError, "Could not save folder", err)
		return
	}

	newFolder.SaveToProject(project.Id)

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
