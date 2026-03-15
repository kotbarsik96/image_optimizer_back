package main

import (
	"crypto/md5"
	"io"
	"maps"
	"os"
	"time"

	"github.com/gin-gonic/gin"
)

type Utils struct{}

func (_ Utils) Md5(s string) []byte {
	h := md5.New()
	io.WriteString(h, s)
	return h.Sum(nil)
}

func (_ Utils) GetCurrentFormattedTime() string {
	return time.Now().Format(time.DateTime)
}

func (_ Utils) GetProjectEnvMode() ProjectEnvMode {
	v := os.Getenv("DEV")
	if v == "1" {
		return ENVMODE_DEV
	}
	return ENVMODE_PROD
}

func (u Utils) AbortWithError(c *gin.Context, code int, safeErrorText, fullErrorText string, jsonObj map[string]any) {
	mode := u.GetProjectEnvMode()

	if jsonObj == nil {
		jsonObj = make(map[string]any)
	}

	switch mode {
	case ENVMODE_PROD:
		maps.Copy(jsonObj, gin.H{
			"error": safeErrorText,
		})
	case ENVMODE_DEV:
		maps.Copy(jsonObj, gin.H{
			"error": fullErrorText,
		})
	}

	c.AbortWithStatusJSON(code, jsonObj)
}
