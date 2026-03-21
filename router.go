package main

import (
	"os"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

var router *gin.Engine
var store cookie.Store

func RouterUp() {
	router = gin.Default()

	RegisterCors()
	RegisterMiddlewares()
	ListRoutes()

	router.Run(os.Getenv("PROJECT_URL"))
}

func RegisterCors() {
	router.Use(cors.New(cors.Config{
		AllowOrigins:     strings.Split(os.Getenv("CORS_ALLOWED_ORIGINS"), ","),
		AllowMethods:     strings.Split(os.Getenv("CORS_ALLOWED_METHODS"), ","),
		AllowHeaders:     strings.Split(os.Getenv("CORS_ALLOWED_HEADERS"), ","),
		ExposeHeaders:    strings.Split(os.Getenv("CORS_EXPOSE_HEADERS"), ","),
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))
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

	optimizations := router.Group("/optimizations", AuthMiddleware())
	{
		optimizations.GET("/:optimization_id", OptimizationAuthMiddleware(), RouteGetOptimization)
		optimizations.DELETE("/:optimization_id", OptimizationAuthMiddleware(), RouteDeleteOptimization)
		optimizations.GET("/list/:project_id", ProjectAuthMiddleware(), RouteGetOptimizationsList)
		optimizations.POST("/start/:project_id", ProjectAuthMiddleware(), RouteStartOptimization)
		optimizations.POST("/rename/:optimization_id", OptimizationAuthMiddleware(), RouteRenameOptimization)
	}
}
