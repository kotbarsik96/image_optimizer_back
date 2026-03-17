package main

import (
	"os"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

type RouterWrapper struct {
	router *gin.Engine
	store  cookie.Store
}

func NewRouterWrapper() RouterWrapper {
	var rw RouterWrapper

	rw.router = gin.Default()

	rw.store = cookie.NewStore([]byte(os.Getenv("SESSION_STORE_KEY")))

	rw.router.Use(sessions.Sessions(os.Getenv("PROJECT_NAME"), rw.store))

	return rw
}

func (rw *RouterWrapper) Up() {
	router := rw.router

	router.GET("/projects", RouteGetProjectsList)
	router.GET("/project/:id", RouteGetProject)

	router.POST("/project/new", RouteNewProject)
	router.POST("/project/:id/create-folder", RouteNewFolder)
	router.POST("/folder/:id/upload", RouteUploadFiles)

	router.DELETE("/project/:id", RouteDeleteProject)
	router.DELETE("/folder/:id", RouteDeleteFolder)
	router.DELETE("/image/:id", RouteDeleteImage)

	router.Run("localhost:8080")
}
