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
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": utils.GetSafeError(fmt.Errorf("Could not get projects"), err),
		})
		return
	}

	for rows.Next() {
		project := TProjectPreview{
			Optimizations: []TOptimizationPreview{},
		}
		err := rows.Scan(&project.Id, &project.Title, &project.CreatedAt, &project.UpdatedAt)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"error": utils.GetSafeError(fmt.Errorf("Error while trying to get project"), err),
			})
			return
		}

		oRows, err := dbwrapper.DB.Query(`
			SELECT id, output_extension, created_at, updated_at
			FROM optimizations
			WHERE project_id = ?
		`, project.Id)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"error": utils.GetSafeError(fmt.Errorf("Could not get optimizations for project %v", project.Id), err),
			})
			return
		}

		for oRows.Next() {
			opt := TOptimizationPreview{}
			err := oRows.Scan(&opt.Id, &opt.OutputExtension, &opt.CreatedAt, &opt.UpdatedAt)
			if err != nil {
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"error": utils.GetSafeError(fmt.Errorf("Error while trying to get optimization for project %v", project.Id), err),
				})
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
	id, _ := strconv.Atoi(idParam)
	project, err := GetProjectEntity(id)
	if err != nil {
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
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": utils.GetSafeError(fmt.Errorf("Could not get project"), err),
		})
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
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": utils.GetSafeError(fmt.Errorf("Could not save project"), err),
		})
		return
	}

	// создание папки ("." - значит в корне проекта)
	folder := NewFolderEntity(uploader.Id, ".")
	err = folder.Save()
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": utils.GetSafeError(fmt.Errorf("Could not save folder"), err),
		})
		return
	}

	// связать папку с проектом
	folder.SaveToProject(project.Id)

	// загрузка изображений на s3 и сохранение в базу данных; изображение будет связано с папкой
	form, _ := c.MultipartForm()
	images := form.File["images"]
	fmt.Println(images)
	responseData := UploadProjectImages(uploader, folder, images)

	c.JSON(http.StatusCreated, gin.H{
		"message": fmt.Sprintf(`Project "%v" was created`, project.Title),
		"data":    responseData,
	})
}

func RouteNewFolder(c *gin.Context) {
	uploader := GetCurrentUploader(c)

	projectId, _ := strconv.Atoi(c.Param("id"))
	project, err := GetProjectEntity(projectId)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{
			"error": utils.GetSafeError(fmt.Errorf("Project not found"), err),
		})
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

	parentId, _ := strconv.Atoi(c.PostForm("parent_id"))
	parentFolder, err := GetFolderEntity(parentId)
	if err != nil {
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
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": utils.GetSafeError(fmt.Errorf("Could not save folder"), err),
		})
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

	folderId, _ := strconv.Atoi(c.Param("id"))
	folder, err := GetFolderEntity(folderId)
	if err != nil {
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

func RouteDeleteProject(c *gin.Context) {
	uploader := GetCurrentUploader(c)

	projectId, _ := strconv.Atoi(c.Param("id"))
	project, err := GetProjectEntity(projectId)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{
			"error": err,
		})
		return
	}

	if project.UploaderId != uploader.Id {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"error": "Unauthorized",
		})
		return
	}

	stmt, err := dbwrapper.DB.Prepare("DELETE FROM projects WHERE id = ?")
	if err == nil {
		_, err = stmt.Exec(project.Id)
	}

	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": utils.GetSafeError(fmt.Errorf("Could not delete project"), err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf(`Project "%v" was deleted`, project.Title),
	})
}

func RouteDeleteFolder(c *gin.Context) {
	uploader := GetCurrentUploader(c)

	folderId, _ := strconv.Atoi(c.Param("id"))
	folder, err := GetFolderEntity(folderId)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{
			"error": err,
		})
		return
	}

	if folder.UploaderId != uploader.Id {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"error": "Unauthorized",
		})
		return
	}

	stmt, err := dbwrapper.DB.Prepare("DELETE FROM folders WHERE id = ?")
	if err == nil {
		_, err = stmt.Exec(folder.Id)
	}

	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": utils.GetSafeError(fmt.Errorf("Could not delete folder"), err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf(`Folder "%v" was deleted`, folder.Path),
	})
}

func RouteDeleteImage(c *gin.Context) {
	uploader := GetCurrentUploader(c)

	imageId, _ := strconv.Atoi(c.Param("id"))
	image, err := GetImageEntity(imageId)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{
			"error": err,
		})
		return
	}

	folder, err := image.GetFolder()
	if err != nil {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{
			"error": err,
		})
		return
	}

	if folder.UploaderId != uploader.Id {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"error": "Unauthorzed",
		})
		return
	}

	stmt, err := dbwrapper.DB.Prepare("DELETE FROM images WHERE id = ?")
	if err == nil {
		_, err = stmt.Exec(image.Id)
	}

	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": utils.GetSafeError(fmt.Errorf("Could not delete image"), err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf(`Image "%v" was deleted`, fmt.Sprintf("%v.%v", image.Filename, image.Extension)),
	})
}
