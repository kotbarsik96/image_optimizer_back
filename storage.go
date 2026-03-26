package main

import (
	"context"
	"errors"
	"fmt"
	"image_optimizer/imgopt_s3"
	"io"
	"log"
	"mime/multipart"
	"os"
	"path"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type EStorage string

const (
	EStorageLocal = "local"
	EStorageS3    = "s3"
)

type IStorage interface {
	Download(ctx context.Context, sourcePath, outputPath string) (*os.File, error)
	PutImage(ctx context.Context, destPath string, file *multipart.FileHeader) error
	Remove(ctx context.Context, destPath string) error
	RemoveFiles(ctx context.Context, destPaths []string) error
}

type StoragesList map[EStorage]IStorage

func (sl StoragesList) Cases() []string {
	cases := []string{}
	for c := range sl {
		cases = append(cases, string(c))
	}
	return cases
}

var Storages StoragesList = StoragesList{
	EStorageLocal: StorageLocal{
		RootPath: os.Getenv("RESOURCES_PATH"),
	},
	EStorageS3: StorageS3{
		Actions:     imgopt_s3.NewS3Action(),
		EndpointUrl: os.Getenv("S3_ENDPOINT_URL"),
		RootPath:    os.Getenv("PROJECT_NAME"),
		Bucket:      os.Getenv("S3_BUCKET"),
	},
}

// локальное хранилище: находится в файловой системе
type StorageLocal struct {
	RootPath string
}

func (storage StorageLocal) Download(ctx context.Context, sourcePath, outputPath string) (*os.File, error) {
	sourceFile, err := os.ReadFile(sourcePath)
	if err != nil {
		return nil, err
	}

	outputFile, err := os.Create(outputPath)
	if err != nil {
		return nil, err
	}

	_, err = outputFile.Write(sourceFile)
	return outputFile, err
}

func (storage StorageLocal) PutImage(ctx context.Context, destPath string, fileHeader *multipart.FileHeader) error {
	file, err := fileHeader.Open()
	if err != nil {
		return err
	}
	defer file.Close()

	dir := path.Dir(destPath)
	filename := GetFilenameWithoutExtension(destPath)
	extension := path.Ext(destPath)

	putDir := path.Join(storage.RootPath, dir)
	putPath := path.Join(putDir, filename+extension)

	err = os.MkdirAll(putDir, 0666)
	if err != nil {
		return err
	}

	createdFile, err := os.Create(putPath)
	if err != nil {
		return err
	}
	defer createdFile.Close()

	_, err = io.Copy(createdFile, file)

	return err
}

func (storage StorageLocal) Remove(ctx context.Context, destPath string) error {
	return os.Remove(destPath)
}

func (storage StorageLocal) RemoveFiles(ctx context.Context, destPaths []string) error {
	if len(destPaths) < 1 {
		return nil
	}

	var err error
	for _, pt := range destPaths {
		err = os.Remove(pt)
	}

	return fmt.Errorf("Not all files are removed. Last error: %w", err)
}

// хранилище S3
type StorageS3 struct {
	Actions     *imgopt_s3.S3Actions
	EndpointUrl string
	RootPath    string
	Bucket      string
}

func (storage StorageS3) Download(ctx context.Context, sourcePath, outputPath string) (*os.File, error) {
	if storage.Actions == nil {
		return nil, ErrS3IsNotAvailable
	}

	client := storage.Actions.Client
	key := path.Join(storage.RootPath, sourcePath)

	result, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(storage.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}
	defer result.Body.Close()

	body, err := io.ReadAll(result.Body)
	if err != nil {
		return nil, err
	}

	outputFile, err := os.Create(outputPath)
	if err != nil {
		return nil, err
	}

	_, err = outputFile.Write(body)
	return outputFile, err
}

func (storage StorageS3) PutImage(ctx context.Context, destPath string, fileHeader *multipart.FileHeader) error {
	if storage.Actions == nil {
		return ErrS3IsNotAvailable
	}

	file, err := fileHeader.Open()
	if err != nil {
		return err
	}

	client := storage.Actions.Client

	key := path.Join(storage.RootPath, destPath)
	extension := path.Ext(destPath)[1:]

	_, err = client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(storage.Bucket),
		Key:         aws.String(key),
		Body:        file,
		ContentType: aws.String("image/" + extension),
	})

	return err
}

func (storage StorageS3) Remove(ctx context.Context, destPath string) error {
	return nil
}

func (storage StorageS3) RemoveFiles(ctx context.Context, destPaths []string) error {
	if storage.Actions == nil {
		return ErrS3IsNotAvailable
	}

	client := storage.Actions.Client

	if len(destPaths) == 0 {
		return nil
	}

	objects := []types.ObjectIdentifier{}
	for _, key := range destPaths {
		objects = append(objects, types.ObjectIdentifier{
			Key: &key,
		})
	}

	input := s3.DeleteObjectsInput{
		Bucket: aws.String(storage.Bucket),
		Delete: &types.Delete{
			Objects: objects,
		},
	}

	delOut, err := client.DeleteObjects(ctx, &input)
	if err != nil || len(delOut.Errors) > 0 {
		// удалить объекты не удалось - получить ошибку
		log.Printf("Error deleting objects from bucket %s\n", storage.Bucket)
		if err != nil {
			var noBucket *types.NoSuchBucket
			if errors.As(err, &noBucket) {
				err = noBucket
			}
		} else if len(delOut.Errors) > 0 {
			for _, outErr := range delOut.Errors {
				log.Printf("%s: %s\n", *outErr.Key, *outErr.Message)
			}
			err = fmt.Errorf("%s", *delOut.Errors[0].Message)
		} else {
			// убедиться в успешном удалении объектов или получить ошибку
			for _, delObjs := range delOut.Deleted {
				err = s3.NewObjectNotExistsWaiter(client).Wait(
					ctx,
					&s3.HeadObjectInput{Bucket: aws.String(storage.Bucket), Key: delObjs.Key},
					time.Minute)
				if err != nil {
					log.Printf("Failed attempt to wait for object %s to be deleted.\n", *delObjs.Key)
				} else {
					log.Printf("Deleted %s.\n", *delObjs.Key)
				}
			}
		}
	}

	return err
}
