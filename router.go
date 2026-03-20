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
		projects.POST("/new", RouteNewProject)
		projects.GET("/:project_id", ProjectAuthMiddleware(), RouteGetProject)
		projects.DELETE("/:project_id", ProjectAuthMiddleware(), RouteDeleteProject)
		projects.POST("/rename/:project_id", ProjectAuthMiddleware(), RouteRenameProject)
	}

	folders := router.Group("/folders", AuthMiddleware())
	{
		folders.GET("/:folder_id", ProjectFolderAuthMiddleware(), RouteGetFolder)
		folders.DELETE("/:folder_id", ProjectFolderAuthMiddleware(), RouteDeleteFolder)
		folders.POST("/new/:parent_folder_id", RouteNewFolder)
		folders.POST("/upload/:folder_id", ProjectFolderAuthMiddleware(), RouteUploadFiles)
		folders.POST("/rename/:folder_id", ProjectFolderAuthMiddleware(), RouteRenameFolder)
	}

	images := router.Group("/images", AuthMiddleware())
	{
		images.DELETE("/:image_id", ProjectImageAuthMiddleware(), RouteDeleteImage)
		images.POST("/rename/:image_id", ProjectImageAuthMiddleware(), RouteRenameImage)
	}
}
