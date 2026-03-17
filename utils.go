package main

import (
	"crypto/md5"
	"fmt"
	"io"
	"os"
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

func (_ Utils) GetEnvMode() ProjectEnvMode {
	v := os.Getenv("DEV")
	if v == "1" {
		return ENVMODE_DEV
	}
	return ENVMODE_PROD
}

func (u Utils) GetSafeError(safeError, describedError error) error {
	mode := u.GetEnvMode()

	switch mode {
	case ENVMODE_PROD:
		return safeError
	case ENVMODE_DEV:
		return fmt.Errorf("%v: %v", safeError, describedError)
	}

	return safeError
}
