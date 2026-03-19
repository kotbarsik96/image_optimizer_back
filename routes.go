package main

import (
	"context"
	"fmt"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func RouteGetProjectsList(c *gin.Context) {
	uploader := c.MustGet("uploader").(Uploader)

	projects := []struct {
		// Optimizations []Optimization
		ID        uint
		Title     string
		CreatedAt time.Time
		UpdatedAt time.Time
	}{}

	gormDb.
		Table("projects").
		Select("id", "title", "created_at", "updated_at").
		Where("uploader_id = ?", uploader.ID).
		Find(&projects)

	RespondOk(c, Response{
		Data: projects,
	})
}

func RouteGetProject(c *gin.Context) {
	project := c.MustGet("project").(Project)

	rootFolder, err := project.RootFolder()
	if err != nil {
		RespondError(c, Response{
			Error: ErrInternal("Could not get project folder", err),
		})
		return
	}
	rootFolder.Nested = rootFolder.GetNested()

	prPreview := ProjectPreview{
		ID:         project.ID,
		CreatedAt:  project.CreatedAt,
		UpdatedAt:  project.UpdatedAt,
		RootFolder: rootFolder,
		Title:      project.Title,
	}

	RespondOk(c, Response{
		Data: prPreview,
	})
}

func RouteNewProject(c *gin.Context) {
	ctx := context.Background()

	uploader := c.MustGet("uploader").(Uploader)

	title := strings.TrimSpace(c.PostForm("title"))
	if title == "" {
		RespondError(c, Response{
			Error: ErrBadRequest("Invalid project name", nil),
		})
		return
	}

	project := Project{
		UploaderID: uploader.ID,
		Title:      c.PostForm("title"),
	}
	err := gorm.G[Project](gormDb).Create(ctx, &project)

	if err != nil {
		RespondError(c, Response{
			Error: ErrInternal("Could not create project", err),
		})
		return
	}

	folder := Folder{
		ProjectID: project.ID,
		Path:      ".",
	}
	err = gorm.G[Folder](gormDb).Create(ctx, &folder)
	if err != nil {
		RespondError(c, Response{
			Error: ErrInternal("Could not create folder", err),
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
	ctx := context.Background()

	uploader := c.MustGet("uploader").(Uploader)
	project := c.MustGet("project").(Project)

	newTitle := strings.TrimSpace(c.PostForm("title"))
	existingProject, err := gorm.G[Project](gormDb).
		Where("title = ? AND uploader_id = ?", newTitle, uploader.ID).
		First(ctx)
	if err == nil && existingProject.Title == newTitle {
		RespondError(c, Response{
			Error: ErrBadRequest(
				fmt.Sprintf("Project %v already exists", existingProject.Title),
				nil),
		})
		return
	}

	project.Title = newTitle
	gormDb.Save(&project)
	if project.Title != newTitle {
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
	ctx := context.Background()

	project := c.MustGet("project").(Project)

	parentId, _ := strconv.Atoi(c.PostForm("parent_id"))
	parentFolder, err := gorm.G[Folder](gormDb).
		Where("id = ?", parentId).
		First(ctx)
	if err != nil {
		RespondError(c, Response{
			Error: ErrBadRequest("Invalid parent folder", err),
		})
		return
	}

	newFolderPath := path.Join(parentFolder.Path, c.PostForm("name"))
	existingFolder, err := gorm.G[Folder](gormDb).
		Where("project_id = ? AND path = ?", project.ID, newFolderPath).
		First(ctx)
	if err == nil && existingFolder.Path == newFolderPath {
		RespondError(c, Response{
			Error: ErrBadRequest(
				fmt.Sprintf("Folder %v already exists in project %v", existingFolder.Path, project.Title),
				err,
			),
		})
		return
	}

	newFolder := Folder{
		Path:      newFolderPath,
		ProjectID: project.ID,
	}
	gormDb.Save(&newFolder)

	RespondCreated(c, Response{
		Data: newFolder,
	})
}

func RouteUploadFiles(c *gin.Context) {
	uploader := c.MustGet("uploader").(Uploader)
	folder := c.MustGet("folder").(Folder)

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
	ctx := context.Background()

	folder := c.MustGet("folder").(Folder)

	if folder.Path == "." {
		RespondError(c, Response{
			Error: ErrBadRequest(
				"Invalid request parameters",
				fmt.Errorf("Cannot rename root folder")),
		})
		return
	}

	project, err := gorm.G[Project](gormDb).
		Where("id = ?", folder.ProjectID).
		First(ctx)
	if err != nil {
		RespondError(c, Response{
			Error: ErrBadRequest("Folder is not associated with any project", err),
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

	existingFolder, err := gorm.G[Folder](gormDb).
		Where("path = ? AND project_id = ?", newPath, project.ID).
		First(ctx)
	if err == nil {
		RespondError(c, Response{
			Error: ErrBadRequest(
				fmt.Sprintf("Folder %v already exists in project %v", existingFolder.Path, project.Title),
				nil),
		})
		return
	}

	folder.Path = newPath
	gormDb.Save(&folder)

	RespondOk(c, Response{
		Message: fmt.Sprintf("Folder renamed to %v (%v)", newName, folder.Path),
		Data: gin.H{
			"folder": folder,
		},
	})
}

func RouteDeleteProject(c *gin.Context) {
	ctx := context.Background()
	project := c.MustGet("project").(Project)

	_, err := gorm.G[Project](gormDb).
		Where("id = ?", project.ID).
		Delete(ctx)
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

func RouteGetFolder(c *gin.Context) {
	ctx := context.Background()

	folder := c.MustGet("folder").(Folder)
	images, err := gorm.G[Image](gormDb).
		Where("folder_id = ?", folder.ID).
		Find(ctx)
	folder.Images = images
	folder.Nested = folder.GetNested()

	if err != nil {
		RespondError(c, Response{
			Error: ErrInternal(
				fmt.Sprintf("Could not retrieve images for folder %v", folder.Path),
				err,
			),
			Data: folder,
		})
		return
	}

	RespondOk(c, Response{
		Data: folder,
	})
}

func RouteDeleteFolder(c *gin.Context) {
	folder := c.MustGet("folder").(Folder)

	err := folder.Delete()
	if err != nil {
		RespondError(c, Response{
			Error: ErrInternal("Could not delete folder", err),
		})
		return
	}

	RespondOk(c, Response{
		Message: fmt.Sprintf(`Folder %v was deleted`, folder.Path),
	})
}

func RouteRenameImage(c *gin.Context) {
	ctx := context.Background()

	img := c.MustGet("image").(Image)
	folder, err := gorm.G[Folder](gormDb).
		Where("id = ?", img.FolderID).
		First(ctx)
	if err != nil {
		RespondError(c, Response{
			Error: ErrInternal("Could not get folder", err),
		})
		return
	}

	newName := strings.TrimSpace(c.PostForm("name"))
	if !IsAcceptablePathName(newName) {
		RespondError(c, Response{
			Error: ErrBadRequest("Invalid filename", nil),
		})
		return
	}

	existingImage, err := gorm.G[Image](gormDb).
		Where("folder_id = ? AND filename = ?", folder.ID, newName).
		First(ctx)
	if err == nil && existingImage.Filename == newName {
		RespondError(c, Response{
			Error: ErrBadRequest(
				fmt.Sprintf("Image %v already exists in folder %v", existingImage.Filename, folder.Path),
				nil),
		})
		return
	}

	img.Filename = newName
	gormDb.Save(&img)

	RespondOk(c, Response{
		Message: fmt.Sprintf("Image renamed to %v", img.Filename),
		Data: gin.H{
			"image": img,
		},
	})
}

func RouteDeleteImage(c *gin.Context) {
	image := c.MustGet("image").(Image)
	err := image.Delete()
	if err != nil {
		RespondError(c, Response{
			Error: ErrInternal("Could not delete image", err),
		})
		return
	}

	RespondOk(c, Response{
		Message: fmt.Sprintf("Image %v was deleted", image.Filename),
	})
}
