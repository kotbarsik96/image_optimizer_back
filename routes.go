package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// projects

func RouteGetProjectsList(c *gin.Context) {
	ctx := c.Request.Context()

	uploader := c.MustGet("uploader").(Uploader)

	projects, err := gorm.G[Project](gormDb).
		Where("uploader_id = ?", uploader.ID).
		Order("created_at desc").
		Find(ctx)
	if err != nil {
		RespondError(c, Response{
			Error: ErrInternal("Could not get projects", err),
		})
		return
	}

	RespondOk(c, Response{
		Data: projects,
	})
}

func RouteNewProject(c *gin.Context) {
	ctx := c.Request.Context()

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

	responseData := []UploadData{}
	if c.ContentType() == "multipart/form-data" {
		form, err := c.MultipartForm()
		if err != nil {
			log.Printf("Could not get multipart form: %v\n", err)
		}

		images := form.File["images"]
		responseData = UploadProjectImages(ctx, uploader, folder, images)
	}

	RespondCreated(c, Response{
		Data: gin.H{
			"uploads": responseData,
			"project": project,
		},
		Message: fmt.Sprintf("Project %v created", project.Title),
	})
}

func RouteGetProject(c *gin.Context) {
	ctx := c.Request.Context()

	project := c.MustGet("project").(Project)

	rootFolder, err := project.GetRootFolder(ctx)
	if err != nil {
		RespondError(c, Response{
			Error: ErrInternal("Could not get project folder", err),
		})
		return
	}
	project.RootFolder = &rootFolder

	optimizations, err := project.GetOptimizations(ctx)
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
	ctx := c.Request.Context()

	project := c.MustGet("project").(Project)

	err := project.Delete(ctx)

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
	ctx := c.Request.Context()

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
	ctx := c.Request.Context()

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
	ctx := c.Request.Context()

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
	fmt.Println(c.PostForm("name"))
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
		Data:    newFolder,
		Message: fmt.Sprintf("Folder %v created", newFolder.Path),
	})
}

func RouteDeleteFolder(c *gin.Context) {
	ctx := c.Request.Context()

	folder := c.MustGet("folder").(Folder)

	if folder.Path == "." {
		RespondError(c, Response{
			Error: ErrBadRequest("Cannot delete root folder", nil),
		})
		return
	}

	err := folder.Delete(ctx)
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
	ctx := c.Request.Context()

	uploader := c.MustGet("uploader").(Uploader)
	folder := c.MustGet("folder").(Folder)

	responseData := []UploadData{}
	if c.ContentType() == "multipart/form-data" {
		form, _ := c.MultipartForm()
		images := form.File["images"]
		responseData = UploadProjectImages(ctx, uploader, folder, images)
	}

	RespondCreated(c, Response{
		Data: gin.H{
			"folder":  folder,
			"uploads": responseData,
		},
	})
}

func RouteRenameFolder(c *gin.Context) {
	ctx := c.Request.Context()

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
	ctx := c.Request.Context()

	image := c.MustGet("image").(Image)
	err := image.Delete(ctx)
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
	ctx := c.Request.Context()

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
		Where("folder_id = ? AND filename = ? AND extension = ?", folder.ID, newName, img.Extension).
		First(ctx)
	if err == nil && existingImage.Filename == newName {
		RespondError(c, Response{
			Error: ErrBadRequest(
				fmt.Sprintf("Image %v.%v already exists in folder %v", existingImage.Filename, existingImage.Extension, folder.Path),
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

	RespondOk(c, Response{
		Data: optimization,
	})
}

func RouteDeleteOptimization(c *gin.Context) {
	ctx := c.Request.Context()

	optimization := c.MustGet("optimization").(Optimization)

	err := optimization.Delete(ctx)
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
	ctx := c.Request.Context()

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
	ctx := c.Request.Context()

	project := c.MustGet("project").(Project)

	// строка в формате "png|webp|avif" или "avif"; каждое из значений должно быть среди ESupportedOptimizationExtensions
	extensionsString := c.PostForm("extensions")
	// строка в формате "25|50|75" или "50"; каждое число не может быть меньше MinOptimizationPercent и больше MaxOptimizationPercent
	sizesString := c.PostForm("sizes")

	_, err := GetOptimizationExtensions(extensionsString)
	if err != nil {
		RespondError(c, Response{
			Error: ErrBadRequest(err.Error(), nil),
		})
		return
	}

	_, err = GetOptimizationSizes(sizesString)
	if err != nil {
		RespondError(c, Response{
			Error: ErrBadRequest(err.Error(), nil),
		})
		return
	}

	title := strings.TrimSpace(c.PostForm("title"))
	if title == "" {
		title = GetCurrentFormattedTime()
	}

	existingOpt, err := gorm.G[Optimization](gormDb).
		Where("project_id = ? AND title = ?", project.ID, title).
		First(ctx)
	if err == nil && existingOpt.Title == title {
		RespondError(c, Response{
			Error: ErrBadRequest(
				fmt.Sprintf("Optimization %v already exists", existingOpt.Title),
				nil),
		})
		return
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

	go opt.Start()

	RespondOkWithCode(
		c,
		Response{
			Message: "Optimization started",
		},
		http.StatusAccepted)
}

func RouteRenameOptimization(c *gin.Context) {
	ctx := c.Request.Context()

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

func RouteDownloadOptimization(c *gin.Context) {
	uploader := c.MustGet("uploader").(Uploader)
	optimization := c.MustGet("optimization").(Optimization)

	zipFilepath := path.Join(os.Getenv("OPTIMIZATIONS_PATH"), uploader.Uuid, optimization.Title+".zip")
	_, err := os.Stat(zipFilepath)
	if err != nil {
		RespondError(c, Response{
			Error: ErrNotFound("Optimization zip not found", err),
		})
		return
	}

	c.FileAttachment(zipFilepath, optimization.Title+".zip")
}
