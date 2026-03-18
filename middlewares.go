package main

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
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
		session := sessions.Default(c)
		idFromSession := session.Get("uploader")
		uploaderUuid := ""
		if idFromSession != nil {
			uploaderUuid = idFromSession.(string)
		}

		uploader := TUploaderEntity{
			Uuid: uploaderUuid,
		}
		err := uploader.GetData()
		if err != nil {
			c.AbortWithStatusJSON(
				http.StatusInternalServerError,
				ErrInternal("Could not get uploader data", err))
			return
		}

		if idFromSession == nil {
			session.Set("uploader", uploader.Uuid)
			session.Save()
		}

		c.Set("uploader", uploader)

		c.Next()
	}
}

func ProjectAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		uploader := c.MustGet("uploader").(TUploaderEntity)
		projectId, _ := strconv.Atoi(c.Param("project_id"))
		project, err := GetProjectEntity(projectId)
		if err != nil {
			c.AbortWithStatusJSON(
				http.StatusNotFound,
				ErrNotFound("Project not found", err))
			return
		}

		if uploader.Id != project.UploaderId {
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
		uploader := c.MustGet("uploader").(TUploaderEntity)
		folderId, _ := strconv.Atoi(c.Param("folder_id"))
		folder, err := GetFolderEntity(folderId)
		if err != nil {
			c.AbortWithStatusJSON(
				http.StatusNotFound,
				ErrNotFound("Folder not found", err))
			return
		}

		project, err := GetProjectEntity(folder.ProjectId)
		if err != nil {
			c.AbortWithStatusJSON(
				http.StatusBadRequest,
				ErrBadRequest("Folder is not attached to project", err))
			return
		}

		if project.UploaderId != uploader.Id {
			c.AbortWithStatusJSON(http.StatusUnauthorized, ErrUnauthorized("", nil))
			return
		}

		c.Set("folder", folder)

		c.Next()
	}
}
