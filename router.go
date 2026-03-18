package main

import (
	"os"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

var router *gin.Engine
var store cookie.Store

func RouterUp() {
	router = gin.Default()

	RegisterMiddlewares()
	ListRoutes()

	router.Run(os.Getenv("PROJECT_URL"))
}

func RegisterMiddlewares() {
	store = cookie.NewStore([]byte(os.Getenv("SESSION_STORE_KEY")))
	session := sessions.Sessions(os.Getenv("PROJECT_NAME"), store)
	router.Use(session)

	router.Use(ErrorMiddleware())
}

func ListRoutes() {
	projects := router.Group("/projects", AuthMiddleware())
	{
		projects.GET("/", RouteGetProjectsList)
		projects.GET("/:project_id", ProjectAuthMiddleware(), RouteGetProject)
		projects.POST("/", RouteNewProject)
		projects.POST("/:project_id/folder", ProjectAuthMiddleware(), RouteNewFolder)
		projects.DELETE("/:project_id", ProjectAuthMiddleware(), RouteDeleteProject)
	}

	folders := router.Group("/folders", AuthMiddleware())
	{
		folders.POST("/:folder_id/upload", ProjectFolderAuthMiddleware(), RouteUploadFiles)
		folders.DELETE("/:folder_id", ProjectFolderAuthMiddleware(), RouteDeleteFolder)
	}

	images := router.Group("/images", AuthMiddleware())
	{
		images.DELETE("/:image_id", RouteDeleteImage)
	}
}
