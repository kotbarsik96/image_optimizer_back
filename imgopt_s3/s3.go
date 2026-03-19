package imgopt_s3

import (
	"context"
	"errors"
	"fmt"
	"log"
	"mime/multipart"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type S3Actions struct {
	Config  *aws.Config
	Client  *s3.Client
	Manager *transfermanager.Client
}

func (s *S3Actions) Init() error {
	err := s.InitConfig()
	if err != nil {
		return err
	}

	s.InitClient()

	return nil
}

func (s *S3Actions) InitConfig() error {
	if s.Config != nil {
		return nil
	}

	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithSharedCredentialsFiles([]string{
			"./.aws/credentials",
		}),
		config.WithSharedConfigFiles([]string{
			"./.aws/config",
		}),
		config.WithRequestChecksumCalculation(aws.RequestChecksumCalculationWhenRequired),
		config.WithResponseChecksumValidation(aws.ResponseChecksumValidationWhenRequired),
	)
	if err == nil {
		s.Config = &cfg
	}
	return err
}

func (s *S3Actions) InitClient() {
	if s.Client != nil {
		return
	}

	cfg := s.Config
	client := s3.NewFromConfig(*cfg)
	s.Client = client
}

func (s *S3Actions) UploadFile(ctx context.Context, bucket, key string, file multipart.File, contentType string) (
	*s3.PutObjectOutput, error) {
	client := s.Client

	output, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		Body:        file,
		ContentType: aws.String(contentType),
	})

	return output, err
}

func (s *S3Actions) DeleteFiles(ctx context.Context, bucket string, keys []string) error {
	if len(keys) == 0 {
		return nil
	}

	client := s.Client

	objects := []types.ObjectIdentifier{}
	for _, key := range keys {
		objects = append(objects, types.ObjectIdentifier{
			Key: &key,
		})
	}

	input := s3.DeleteObjectsInput{
		Bucket: aws.String(bucket),
		Delete: &types.Delete{
			Objects: objects,
		},
	}

	delOut, err := client.DeleteObjects(ctx, &input)
	if err != nil || len(delOut.Errors) > 0 {
		// удалить объекты не удалось - получить ошибку
		log.Printf("Error deleting objects from bucket %s\n", bucket)
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
				err = s3.NewObjectNotExistsWaiter(s.Client).Wait(
					ctx,
					&s3.HeadObjectInput{Bucket: aws.String(bucket), Key: delObjs.Key},
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

func (s *S3Actions) IsAvailable() error {
	client := s.Client

	_, err := client.HeadBucket(context.TODO(), &s3.HeadBucketInput{
		Bucket: aws.String(os.Getenv("S3_BUCKET")),
	})
	if err != nil {
		return err
	}

	return nil
}
