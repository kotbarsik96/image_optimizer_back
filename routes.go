package main

import (
	"fmt"
	"path"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

func RouteGetProjectsList(c *gin.Context) {
	uploader := c.MustGet("uploader").(TUploaderEntity)

	projects := []TProjectPreview{}

	rows, err := dbwrapper.DB.Query(`
		SELECT id, title, created_at, updated_at
		FROM projects
		WHERE uploader_id = ?
	`, uploader.Id)
	if err != nil {
		RespondError(c, Response{
			Error: ErrInternal("Could not get projects", err),
		})
		return
	}

	for rows.Next() {
		project := TProjectPreview{
			Optimizations: []TOptimizationPreview{},
		}
		err := rows.Scan(&project.Id, &project.Title, &project.CreatedAt, &project.UpdatedAt)
		if err != nil {
			RespondError(c, Response{
				Error: ErrInternal("Could not get project", err),
			})
			return
		}

		oRows, err := dbwrapper.DB.Query(`
			SELECT id, output_extension, created_at, updated_at
			FROM optimizations
			WHERE project_id = ?
		`, project.Id)
		if err != nil {
			RespondError(c, Response{
				Error: ErrInternal("Could not get optimizations", err),
			})
			return
		}

		for oRows.Next() {
			opt := TOptimizationPreview{}
			err := oRows.Scan(&opt.Id, &opt.OutputExtension, &opt.CreatedAt, &opt.UpdatedAt)
			if err != nil {
				RespondError(c, Response{
					Error: ErrInternal(
						fmt.Sprintf("Colud not get optimization for project %v", project.Id),
						err),
				})
				return
			}

			project.Optimizations = append(project.Optimizations, opt)
		}

		projects = append(projects, project)
	}

	RespondOk(c, Response{
		Data: projects,
	})
}

func RouteGetProject(c *gin.Context) {
	project := c.MustGet("project").(TProjectEntity)

	tree, err := project.GetFoldersTree()
	if err != nil {
		RespondError(c, Response{
			Error: ErrInternal("Could not get project folders", err),
		})
		return
	}

	RespondOk(c, Response{
		Data: tree,
	})
}

func RouteNewProject(c *gin.Context) {
	uploader := c.MustGet("uploader").(TUploaderEntity)

	project, err := NewProjectEntity(uploader, c.PostForm("title"))
	if err != nil {
		RespondError(c, Response{
			Error: ErrBadRequest(err.Error(), nil),
		})
		return
	}
	_, err = project.Save()
	if err != nil {
		RespondError(c, Response{
			Error: ErrInternal("Could not save project", err),
		})
		return
	}

	folder, err := project.CreateFolderEntity(".")
	if err != nil {
		RespondError(c, Response{
			Error: ErrBadRequest(err.Error(), nil),
		})
		return
	}

	err = folder.Save()
	if err != nil {
		RespondError(c, Response{
			Error: ErrInternal("Could not save folder", err),
		})
		return
	}

	form, _ := c.MultipartForm()
	images := form.File["images"]
	responseData := UploadProjectImages(uploader, folder, images)

	RespondCreated(c, Response{
		Data: gin.H{
			"uploads": responseData,
			"project": project,
		},
		Message: fmt.Sprintf("Project %v created", project.Title),
	})
}

func RouteRenameProject(c *gin.Context) {
	uploader := c.MustGet("uploader").(TUploaderEntity)
	project := c.MustGet("project").(TProjectEntity)

	newTitle := c.PostForm("title")
	var existingTitle string
	err := dbwrapper.DB.QueryRow("SELECT title FROM projects WHERE title = ? AND uploader_id = ?", newTitle, uploader.Id).
		Scan(&existingTitle)
	if err == nil {
		RespondError(c, Response{
			Error: ErrBadRequest(
				fmt.Sprintf("Project %v already exists", existingTitle),
				nil),
		})
		return
	}

	project.Title = newTitle
	_, err = project.Save()
	if err != nil {
		RespondError(c, Response{
			Error: ErrInternal("Could not save project", err),
		})
		return
	}

	RespondOk(c, Response{
		Message: fmt.Sprintf("Project title set to %v", project.Title),
		Data:    project,
	})
}

func RouteNewFolder(c *gin.Context) {
	project := c.MustGet("project").(TProjectEntity)

	parentId, _ := strconv.Atoi(c.PostForm("parent_id"))
	parentFolder, err := GetFolderEntity(parentId)
	if err != nil {
		RespondError(c, Response{
			Error: ErrBadRequest("Invalid parent folder", err),
		})
		return
	}

	newFolderPath := path.Join(parentFolder.Path, c.PostForm("name"))

	newFolder, err := project.CreateFolderEntity(newFolderPath)
	if err != nil {
		RespondError(c, Response{
			Error: ErrBadRequest(err.Error(), nil),
		})
		return
	}

	err = newFolder.Save()
	if err != nil {
		RespondError(c, Response{
			Error: ErrInternal("Could not save folder", err),
		})
		return
	}

	RespondCreated(c, Response{
		Data: newFolder,
	})
}

func RouteUploadFiles(c *gin.Context) {
	uploader := c.MustGet("uploader").(TUploaderEntity)
	folder := c.MustGet("folder").(TFolderEntity)

	form, _ := c.MultipartForm()
	images := form.File["images"]
	responseData := UploadProjectImages(uploader, folder, images)

	RespondCreated(c, Response{
		Data: gin.H{
			"folder":  folder,
			"uploads": responseData,
		},
	})
}

func RouteRenameFolder(c *gin.Context) {
	folder := c.MustGet("folder").(TFolderEntity)

	project, err := GetProjectEntity(folder.ProjectId)
	if err != nil {
		RespondError(c, Response{
			Error: ErrBadRequest("Folder is not associated with any project", err),
		})
		return
	}

	if folder.Path == "." {
		RespondError(c, Response{
			Error: ErrBadRequest(
				"Invalid request parameters",
				fmt.Errorf("Cannot rename root folder")),
		})
		return
	}

	newName := strings.TrimSpace(c.PostForm("name"))
	if !IsAcceptablePathName(newName) {
		RespondError(c, Response{
			Error: ErrBadRequest("Invalid folder name", nil),
		})
		return
	}

	newPath := path.Join(path.Dir(folder.Path), newName)

	var existingPath string
	err = dbwrapper.DB.QueryRow("SELECT path FROM folders WHERE path = ? AND project_id = ?", newPath, project.Id).
		Scan(&existingPath)
	if err == nil {
		RespondError(c, Response{
			Error: ErrBadRequest(
				fmt.Sprintf("Folder %v already exists in project %v", existingPath, project.Title),
				nil),
		})
		return
	}

	folder.Path = newPath
	err = folder.Save()
	if err != nil {
		RespondError(c, Response{
			Error: ErrBadRequest(
				"Could not save folder",
				err),
		})
		return
	}

	RespondOk(c, Response{
		Message: fmt.Sprintf("Folder renamed to %v (%v)", newName, folder.Path),
		Data: gin.H{
			"project": project,
			"folder":  folder,
		},
	})
}

func RouteDeleteProject(c *gin.Context) {
	project := c.MustGet("project").(TProjectEntity)
	fmt.Println(project)

	stmt, err := dbwrapper.DB.Prepare("DELETE FROM projects WHERE id = ?")
	if err == nil {
		_, err = stmt.Exec(project.Id)
	}

	if err != nil {
		RespondError(c, Response{
			Error: ErrInternal("Could not delete project", err),
		})
		return
	}

	RespondOk(c, Response{
		Message: fmt.Sprintf(`Project "%v" was deleted`, project.Title),
	})
}

func RouteDeleteFolder(c *gin.Context) {
	folder := c.MustGet("folder").(TFolderEntity)

	stmt, err := dbwrapper.DB.Prepare("DELETE FROM folders WHERE id = ?")
	if err == nil {
		_, err = stmt.Exec(folder.Id)
	}

	if err != nil {
		RespondError(c, Response{
			Error: ErrInternal("Could not delete folder", err),
		})
		return
	}

	RespondOk(c, Response{
		Message: fmt.Sprintf(`Folder "%v" was deleted`, folder.Path),
	})
}

func RouteRenameImage(c *gin.Context) {
	img := c.MustGet("image").(TImageEntity)
	folder, err := GetFolderEntity(img.FolderId)
	if err != nil {
		RespondError(c, Response{
			Error: ErrInternal("Could not get folder", err),
		})
		return
	}

	newName := c.PostForm("name")
	if !IsAcceptablePathName(newName) {
		RespondError(c, Response{
			Error: ErrBadRequest("Invalid filename", nil),
		})
		return
	}

	var existingName string
	err = dbwrapper.DB.QueryRow("SELECT filename FROM images WHERE images.folder_id = ? AND images.filename = ?", folder.Id, newName).
		Scan(&existingName)
	if err == nil {
		RespondError(c, Response{
			Error: ErrBadRequest(
				fmt.Sprintf("Image %v already exists in folder %v", existingName, folder.Path),
				nil),
		})
		return
	}

	img.Filename = newName
	err = img.Save()
	if err != nil {
		RespondError(c, Response{
			Error: ErrInternal("Could not save image", err),
		})
		return
	}

	RespondOk(c, Response{
		Message: fmt.Sprintf("Image renamed to %v", img.Filename),
		Data: gin.H{
			"image": img,
		},
	})
}

func RouteDeleteImage(c *gin.Context) {
	image := c.MustGet("image").(TImageEntity)

	stmt, err := dbwrapper.DB.Prepare("DELETE FROM images WHERE id = ?")
	if err == nil {
		_, err = stmt.Exec(image.Id)
	}

	if err != nil {
		RespondError(c, Response{
			Error: ErrInternal("Could not delete image", err),
		})
		return
	}

	RespondOk(c, Response{
		Message: fmt.Sprintf(`Image %v was deleted`, image.Filename),
	})
}
