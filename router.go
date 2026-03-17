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

	router.Use(ErrorHandler())
}

func ListRoutes() {
	projects := router.Group("/projects", AuthHandler())
	{
		projects.GET("/", RouteGetProjectsList)
		projects.GET("/:project_id", ProjectAuthHandler(), RouteGetProject)
		projects.POST("/", RouteNewProject)
		projects.POST("/:project_id/folder", ProjectAuthHandler(), RouteNewFolder)
		projects.DELETE("/:project_id", ProjectAuthHandler(), RouteDeleteProject)
	}

	folders := router.Group("/folders", AuthHandler())
	{
		folders.POST("/:folder_id/upload", ProjectFolderAuthHandler(), RouteUploadFiles)
		folders.DELETE("/:folder_id", ProjectFolderAuthHandler(), RouteDeleteFolder)
	}

	images := router.Group("/images", AuthHandler())
	{
		images.DELETE("/:image_id", RouteDeleteImage)
	}
}
