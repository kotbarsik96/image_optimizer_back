package main

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// projects

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

func RouteNewProject(c *gin.Context) {
	ctx := context.Background()

	uploader := c.MustGet("uploader").(Uploader)

	title := strings.TrimSpace(c.PostForm("title"))
	if title == "" {
		title = GetCurrentFormattedTime()
	}

	project := Project{
		UploaderID: uploader.ID,
		Title:      title,
	}
	err := gorm.G[Project](gormDb).Create(ctx, &project)

	if err != nil {
		RespondError(c, Response{
			Error: ErrInternal("Could not create project", err),
		})
		return
	}

	folder := Folder{
		ProjectID: &project.ID,
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

func RouteGetProject(c *gin.Context) {
	project := c.MustGet("project").(Project)

	rootFolder, err := project.GetRootFolder()
	if err != nil {
		RespondError(c, Response{
			Error: ErrInternal("Could not get project folder", err),
		})
		return
	}
	project.RootFolder = &rootFolder

	optimizations, err := project.GetOptimizations()
	if err != nil {
		RespondError(c, Response{
			Error: ErrInternal("Could not get optimizations", err),
		})
		return
	}
	project.Optimizations = optimizations

	RespondOk(c, Response{
		Data: project,
	})
}

func RouteDeleteProject(c *gin.Context) {
	project := c.MustGet("project").(Project)

	err := project.Delete()

	if err != nil {
		RespondError(c, Response{
			Error: ErrInternal("Could not delete project", err),
		})
		return
	}

	RespondOk(c, Response{
		Message: fmt.Sprintf(`Project %v was deleted`, project.Title),
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

// folders

func RouteGetFolder(c *gin.Context) {
	ctx := context.Background()

	folder := c.MustGet("folder").(Folder)
	images, imagesErr := gorm.G[Image](gormDb).
		Where("folder_id = ?", folder.ID).
		Find(ctx)
	folder.Images = images

	nestedFolders, nestedFoldersErr := folder.GetNested(ctx)
	folder.Nested = nestedFolders

	if imagesErr != nil || nestedFoldersErr != nil {
		var msg string
		if imagesErr != nil && nestedFoldersErr != nil {
			msg = "Could not contents of folder"
		} else if imagesErr != nil {
			msg = "Could not images"
		} else if nestedFoldersErr != nil {
			msg = "Could not get folders"
		}

		RespondError(c, Response{
			Error: ErrInternal(msg, fmt.Errorf("%v and %v", imagesErr, nestedFoldersErr)),
		})
		return
	}

	RespondOk(c, Response{
		Data: folder,
	})
}

func RouteNewFolder(c *gin.Context) {
	ctx := context.Background()

	uploader := c.MustGet("uploader").(Uploader)

	parentId, _ := strconv.Atoi(c.Param("parent_folder_id"))
	parentFolder, err := gorm.G[Folder](gormDb).
		Where("id = ?", parentId).
		First(ctx)
	if err != nil {
		RespondError(c, Response{
			Error: ErrNotFound("Parent folder not found", err),
		})
		return
	}

	project, err := gorm.G[Project](gormDb).Where("id = ?", parentFolder.ProjectID).First(ctx)
	if err != nil {
		RespondError(c, Response{
			Error: ErrBadRequest("Parent folder is not assigned to any project", err),
		})
		return
	}

	if project.UploaderID != uploader.ID {
		RespondError(c, Response{
			Error: ErrUnauthorized("", nil),
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
		ProjectID: &project.ID,
		ParentID:  &parentFolder.ID,
	}
	gormDb.Save(&newFolder)

	RespondCreated(c, Response{
		Data: newFolder,
	})
}

func RouteDeleteFolder(c *gin.Context) {
	folder := c.MustGet("folder").(Folder)

	if folder.Path == "." {
		RespondError(c, Response{
			Error: ErrBadRequest("Cannot delete root folder", nil),
		})
		return
	}

	err := folder.Delete()
	if err != nil {
		if errors.Is(err, ErrCannotDeleteRootFolder) {
			RespondError(c, Response{
				Error: ErrBadRequest(err.Error(), nil),
			})
		} else {
			RespondError(c, Response{
				Error: ErrInternal("Could not delete folder", err),
			})
		}
		return
	}

	RespondOk(c, Response{
		Message: fmt.Sprintf(`Folder %v was deleted`, folder.Path),
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

// images

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

// optimizations

func RouteGetOptimization(c *gin.Context) {
	optimization := c.MustGet("optimization").(Optimization)

	rootFolder, err := optimization.GetRootFolder()
	if err != nil {
		RespondError(c, Response{
			Error: ErrInternal("Could not get optimization folder", err),
		})
		return
	}
	optimization.RootFolder = rootFolder

	RespondOk(c, Response{
		Data: optimization,
	})
}

func RouteDeleteOptimization(c *gin.Context) {
	optimization := c.MustGet("optimization").(Optimization)

	err := optimization.Delete()
	if err != nil {
		RespondError(c, Response{
			Error: ErrInternal("Could not delete optimization", err),
		})
		return
	}

	RespondOk(c, Response{
		Message: fmt.Sprintf("Optimization %v was deleted", optimization.Title),
	})
}

func RouteGetOptimizationsList(c *gin.Context) {
	ctx := context.Background()

	project := c.MustGet("project").(Project)

	optimizations, err := gorm.G[Optimization](gormDb).Where("project_id = ?", project.ID).Find(ctx)
	if err != nil {
		RespondError(c, Response{
			Error: ErrInternal("Could not get optimizations", err),
		})
		return
	}

	RespondOk(c, Response{
		Data: optimizations,
	})
}

func RouteStartOptimization(c *gin.Context) {
	ctx := context.Background()

	project := c.MustGet("project").(Project)

	// строка в формате "png|webp|avif" или "avif"; каждое из значений должно быть среди ESupportedOptimizationExtensions
	extensionsString := c.PostForm("extensions")
	// строка в формате "25|50|75" или "50"; каждое число не может быть меньше MinOptimizationPercent и больше MaxOptimizationPercent
	sizesString := c.PostForm("sizes")

	extensions, err := GetOptimizationExtensions(extensionsString)
	if err != nil {
		RespondError(c, Response{
			Error: ErrBadRequest(err.Error(), nil),
		})
		return
	}

	sizes, err := GetOptimizationSizes(sizesString)
	if err != nil {
		RespondError(c, Response{
			Error: ErrBadRequest(err.Error(), nil),
		})
		return
	}

	// temp
	fmt.Println(extensions)
	fmt.Println(sizes)
	// temp ^

	title := strings.TrimSpace(c.PostForm("title"))
	if title == "" {
		title = GetCurrentFormattedTime()
	}

	opt := Optimization{
		ProjectID:  project.ID,
		Title:      title,
		Extensions: extensionsString,
		Sizes:      sizesString,
	}

	err = gorm.G[Optimization](gormDb).Create(ctx, &opt)
	if err != nil {
		RespondError(c, Response{
			Error: ErrInternal("Could not save optimization", err),
		})
		return
	}

	err = opt.Start()

	if err != nil {
		RespondError(c, Response{
			Error: ErrInternal("Could not start optimization", err),
		})
		return
	}

	RespondCreated(c, Response{
		Message: "Optimization started",
		Data:    opt,
	})
}

func RouteRenameOptimization(c *gin.Context) {
	ctx := context.Background()

	project := c.MustGet("project").(Project)
	optimization := c.MustGet("optimization").(Optimization)

	newTitle := strings.TrimSpace(c.PostForm("title"))
	existingOpt, err := gorm.G[Optimization](gormDb).
		Where("title = ? AND project_id = ?", newTitle, project.ID).
		First(ctx)
	if err == nil && existingOpt.Title == newTitle {
		RespondError(c, Response{
			Error: ErrBadRequest(
				fmt.Sprintf("Optimization %v already exists", existingOpt.Title),
				nil),
		})
		return
	}

	optimization.Title = newTitle
	gormDb.Save(&optimization)
	if optimization.Title != newTitle {
		RespondError(c, Response{
			Error: ErrInternal("Could not save optimization", err),
		})
		return
	}

	RespondOk(c, Response{
		Message: fmt.Sprintf("Optimization title set to %v", optimization.Title),
		Data:    optimization,
	})
}
