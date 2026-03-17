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

	store = cookie.NewStore([]byte(os.Getenv("SESSION_STORE_KEY")))
	session := sessions.Sessions(os.Getenv("PROJECT_NAME"), store)
	router.Use(session)

	ListRoutes()

	router.Run(os.Getenv("PROJECT_URL"))
}

func ListRoutes() {
	router.GET("/projects", RouteGetProjectsList)
	router.GET("/project/:id", RouteGetProject)

	router.POST("/project/new", RouteNewProject)
	router.POST("/project/:id/create-folder", RouteNewFolder)
	router.POST("/folder/:id/upload", RouteUploadFiles)

	router.DELETE("/project/:id", RouteDeleteProject)
	router.DELETE("/folder/:id", RouteDeleteFolder)
	router.DELETE("/image/:id", RouteDeleteImage)
}
