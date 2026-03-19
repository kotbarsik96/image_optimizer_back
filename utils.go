package main

import (
	"crypto/md5"
	"fmt"
	"io"
	"strings"
)

func Md5(s string) string {
	h := md5.New()
	io.WriteString(h, s)
	return fmt.Sprintf("%x", h.Sum(nil))
}

func IsAcceptablePathName(name string) bool {
	if !foldernameRegExp.MatchString(name) {
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

func GetFilenameWithoutExtension(fname string) string {
	s := strings.Split(fname, ".")
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
