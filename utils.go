package main

import (
	"crypto/md5"
	"fmt"
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

func (_ Utils) GetEnvMode() ProjectEnvMode {
	v := os.Getenv("DEV")
	if v == "1" {
		return ENVMODE_DEV
	}
	return ENVMODE_PROD
}

func (u Utils) AbortWithError(c *gin.Context, code int, text string, err error, jsonObjects ...map[string]any) {
	mode := u.GetEnvMode()

	jsonObj := make(map[string]any)
	for _, jo := range jsonObjects {
		maps.Copy(jsonObj, jo)
	}

	switch mode {
	case ENVMODE_PROD:
		maps.Copy(jsonObj, gin.H{
			"error": text,
		})
	case ENVMODE_DEV:
		maps.Copy(jsonObj, gin.H{
			"error": fmt.Sprintf("%v: %v", text, err),
		})
	}

	c.AbortWithStatusJSON(code, jsonObj)
}
