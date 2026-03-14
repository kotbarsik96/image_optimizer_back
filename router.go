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

	router.POST("/new-project", RouteNewProject)

	router.Run("localhost:8080")
}
