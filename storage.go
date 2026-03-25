package main

import (
	"io"
	"mime/multipart"
	"os"
	"path"
)

type EStorage int

const (
	EStorageLocal = iota
	EStorageS3
)

func (s EStorage) String() string {
	switch s {
	case EStorageLocal:
		return "local"
	case EStorageS3:
		return "s3"
	}

	return "unknown"
}

type StorageWrapper struct {
	Local StorageLocal
	S3    StorageS3
}

var Storage StorageWrapper = StorageWrapper{
	Local: StorageLocal{
		RootPath: os.Getenv("RESOURCES_PATH"),
	},
	S3: StorageS3{
		EndpointUrl: os.Getenv("S3_ENDPOINT_URL"),
		RootPath:    os.Getenv("PROJECT_NAME"),
		Bucket:      os.Getenv("S3_BUCKET"),
	},
}

// локальное хранилище: находится в файловой системе
type StorageLocal struct {
	RootPath string
}

func (storage *StorageLocal) Get(destPath string) {

}

func (storage *StorageLocal) PutImage(destPath string, file *multipart.File) (*os.File, error) {
	dir := path.Dir(destPath)
	filename := GetFilenameWithoutExtension(destPath)
	extension := path.Ext(destPath)

	putDir := path.Join(storage.RootPath, dir)
	putPath := path.Join(putDir, filename+"."+extension)

	err := os.MkdirAll(putDir, 0666)
	if err != nil {
		return nil, err
	}

	createdFile, err := os.Create(putPath)
	if err != nil {
		return createdFile, err
	}
	defer createdFile.Close()

	_, err = io.Copy(createdFile, *file)

	return createdFile, err
}

func (storage *StorageLocal) Remove(destPath string) {

}

// хранилище S3
type StorageS3 struct {
	EndpointUrl string
	RootPath    string
	Bucket      string
}

func (storage *StorageS3) Get(destPath string) {

}

func (storage *StorageS3) Put(destPath string) {

}

func (storage *StorageS3) Remove(destPath string) {

}
