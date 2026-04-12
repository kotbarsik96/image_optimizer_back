package main

import (
	"crypto/md5"
	"fmt"
	"io"
	"math"
	"os"
	"path"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

var acceptablePathNameRegexp = regexp.MustCompile(`^[\pL\pM\pN._ -]+$`)
var unacceptablePathSymbolsRegexp = regexp.MustCompile(`[^\pL\pM\pN._ -]+`)

func Md5(s string) string {
	h := md5.New()
	io.WriteString(h, s)
	return fmt.Sprintf("%x", h.Sum(nil))
}

func IsAcceptablePathName(name string) bool {
	if !acceptablePathNameRegexp.MatchString(name) {
		return false
	}

	if strings.ReplaceAll(name, ".", "") == "" {
		return false
	}

	if strings.HasSuffix(name, ".") || strings.HasSuffix(name, " ") {
		return false
	}

	base := strings.ToUpper(name)
	if i := strings.IndexByte(base, '.'); i >= 0 {
		base = base[:i]
	}

	switch base {
	case "CON", "PRN", "AUX", "NUL",
		"COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9",
		"LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9":
		return false
	}

	return true
}

func ToAcceptablePathName(name string) string {
	acceptable := strings.TrimSpace(unacceptablePathSymbolsRegexp.ReplaceAllString(name, "_"))

	if strings.ReplaceAll(acceptable, ".", "") == "" {
		acceptable = "_"
	}

	for {
		if strings.HasSuffix(acceptable, ".") {
			acceptable = acceptable[:len(acceptable)-1]
		} else {
			break
		}
	}

	base := strings.ToUpper(acceptable)
	dotIndex := strings.IndexByte(base, '.')
	if dotIndex >= 0 {
		base = base[:dotIndex]
	}

	switch base {
	case "CON", "PRN", "AUX", "NUL",
		"COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9",
		"LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9":
		acceptable = fmt.Sprintf("_%v", base[dotIndex:])
	}

	return acceptable
}

func GetFilenameWithoutExtension(fname string) string {
	base := path.Base(fname)
	s := strings.Split(base, ".")
	return strings.Join(s[0:len(s)-1], ".")
}

func FilterSlice[V any](slice []V, filterFunc func(index int, item V, slice []V) bool) []V {
	newSlice := []V{}
	for index, item := range slice {
		if filterFunc(index, item, newSlice) {
			newSlice = append(newSlice, item)
		}
	}
	return newSlice
}

func GetCurrentFormattedTime() string {
	return time.Now().Format(time.DateTime)
}

func CopyFile(destPath, srcPath string) error {
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	destFile, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, srcFile)
	if err != nil {
		return err
	}

	return destFile.Sync()
}

func IsDirEmpty(dirPath string) (bool, error) {
	dir, err := os.Open(dirPath)
	if err != nil {
		return false, err
	}
	defer dir.Close()

	_, err = dir.Readdirnames(1)
	if err == io.EOF {
		return true, nil
	}
	return false, err
}

// получает на вход ext, приводя его в нижний регистр и убирая точку в начале, если есть. Приводит одноимённые расширения в единый формат (jpeg -> jpg). Если fixedExt не содержится в ESupportedOptimizationExtensions - вернёт err ErrNotSupportedExtension. Независимо от наличия ошибки, возвращает преобразованное ext как fixedExt
func ToSupportedExtension(ext string) (fixedExt string, err error) {
	ext = strings.ToLower(ext)

	if ext[0] == '.' {
		ext = ext[1:]
	}

	switch ext {
	case "jpeg":
		ext = "jpg"
	}

	if !slices.Contains(ESupportedOptimizationExtensions, ext) {
		return ext, ErrNotSupportedExtension
	}

	return ext, nil
}

func SSEStream(c *gin.Context, progress *Progress) {
	clientChannel := make(ProgressClientChan)

	select {
	case progress.Stream.NewClients <- clientChannel:
	case <-c.Request.Context().Done():
	case <-time.After(2 * time.Second):
		RespondError(c, Response{
			Error: ErrGatewayTimeout("", nil),
		})
		return
	}

	go func() {
		<-c.Request.Context().Done()

		for range clientChannel {
		}

		select {
		case progress.Stream.ClosedClients <- clientChannel:
		default:
		}
	}()

	c.SSEvent("message", TProgressSSE{
		Value:   math.Round(progress.GetPercent()*100) / 100,
		Details: progress.Details,
	})
	c.Writer.Flush()

	c.Stream(func(w io.Writer) bool {
		if value, ok := <-clientChannel; ok {
			c.SSEvent("message", TProgressSSE{
				Value:   math.Round(value*100) / 100,
				Details: progress.Details,
			})
			return value < 100
		}
		return false
	})
}
