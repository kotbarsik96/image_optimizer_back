package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func ErrorMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if len(c.Errors) == 0 {
			return
		}

		err := c.Errors.Last()

		var appErr *AppError
		if errors.As(err, &appErr) {
			c.JSON(appErr.Status, gin.H{
				"ok": false,
				"error": gin.H{
					"code":    appErr.Code,
					"message": appErr.Error(),
				},
			})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{
				"ok": false,
				"error": gin.H{
					"code":    "SERVER_ERROR",
					"message": "Internal server error",
				},
			})
		}
	}
}

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := context.Background()

		session := sessions.Default(c)

		var uploaderUuid string
		uuidFromSession := session.Get("uploader")

		uploader, err := gorm.G[Uploader](gormDb).Where("uuid = ?", uuidFromSession).First(ctx)
		if err != nil || uuidFromSession == nil {
			if uuidFromSession == nil {
				uploaderUuid = uuid.NewString()
			} else {
				uploaderUuid = uuidFromSession.(string)
			}

			uploader = Uploader{
				Uuid: uploaderUuid,
			}

			err = gorm.G[Uploader](gormDb).Create(ctx, &uploader)

			session.Set("uploader", uploader.Uuid)
			session.Save()
		}

		c.Set("uploader", uploader)

		c.Next()
	}
}

func ProjectAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		uploader := c.MustGet("uploader").(Uploader)
		projectId, _ := strconv.Atoi(c.Param("project_id"))
		project, err := gorm.G[Project](gormDb).Where("id = ?", projectId).First(context.Background())
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				c.AbortWithStatusJSON(
					http.StatusNotFound,
					ErrNotFound("Project not found", err))
			} else {
				c.AbortWithStatusJSON(
					http.StatusInternalServerError,
					ErrInternal("", err))
			}

			return
		}

		if uploader.ID != project.UploaderID {
			c.AbortWithStatusJSON(
				http.StatusUnauthorized,
				ErrUnauthorized("", err),
			)
			return
		}

		c.Set("project", project)

		c.Next()
	}
}

func ProjectFolderAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := context.Background()

		uploader := c.MustGet("uploader").(Uploader)

		folderId, _ := strconv.Atoi(c.Param("folder_id"))
		folder, err := gorm.G[Folder](gormDb).Where("id = ?", folderId).First(ctx)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				c.AbortWithStatusJSON(
					http.StatusNotFound,
					ErrNotFound("Folder not found", err))
			} else {
				c.AbortWithStatusJSON(
					http.StatusInternalServerError,
					ErrInternal("", nil))
			}

			return
		}

		project, err := gorm.G[Project](gormDb).Where("id = ?", folder.ProjectID).First(ctx)
		if err != nil {
			c.AbortWithStatusJSON(
				http.StatusBadRequest,
				ErrBadRequest("Folder is not attached to project", err))
			return
		}

		if project.UploaderID != uploader.ID {
			c.AbortWithStatusJSON(http.StatusUnauthorized, ErrUnauthorized("", nil))
			return
		}

		c.Set("folder", folder)

		c.Next()
	}
}

func ProjectImageAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := context.Background()

		uploader := c.MustGet("uploader").(Uploader)
		imageId, _ := strconv.Atoi(c.Param("image_id"))

		image, err := gorm.G[Image](gormDb).Where("id = ?", imageId).First(ctx)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				c.AbortWithStatusJSON(http.StatusNotFound, ErrNotFound("Image not found", err))
			} else {
				c.AbortWithStatusJSON(http.StatusInternalServerError, ErrInternal("", err))
			}

			return
		}

		var uploaderId uint
		err = sqlDb.QueryRow(`
			SELECT uploaders.id FROM uploaders
			JOIN projects ON projects.uploader_id = uploaders.id
			JOIN folders ON folders.project_id = projects.id
			JOIN images ON images.folder_id = folders.id
			WHERE images.id = ?
		`, imageId).Scan(&uploaderId)

		if err != nil || (uploader.ID != uploaderId && uploaderId != 0) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, ErrUnauthorized("", nil))
			return
		}

		c.Set("image", image)

		c.Next()
	}
}

func OptimizationAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := context.Background()

		uploader := c.MustGet("uploader").(Uploader)

		optimizationId, _ := strconv.Atoi(c.Param("optimization_id"))
		opt, err := gorm.G[Optimization](gormDb).Where("id = ?", optimizationId).First(ctx)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				c.AbortWithStatusJSON(
					http.StatusNotFound,
					ErrNotFound("Optimization not found", err))
			} else {
				c.AbortWithStatusJSON(
					http.StatusInternalServerError,
					ErrInternal("", err))
			}

			return
		}

		project, err := gorm.G[Project](gormDb).Where("id = ?", opt.ProjectID).First(ctx)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				c.AbortWithStatusJSON(
					http.StatusBadRequest,
					ErrBadRequest(
						fmt.Sprintf("Optimization %v is not related to any project", opt.Title),
						err))
			} else {
				c.AbortWithStatusJSON(
					http.StatusInternalServerError,
					ErrInternal("", err))
			}

			return
		}

		if project.UploaderID != uploader.ID {
			c.AbortWithStatusJSON(http.StatusUnauthorized, ErrUnauthorized("", nil))
			return
		}

		c.Set("project", project)
		c.Set("optimization", opt)

		c.Next()
	}
}
