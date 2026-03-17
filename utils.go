package main

import (
	"crypto/md5"
	"io"
	"time"
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
