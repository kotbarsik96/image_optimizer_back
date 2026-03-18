package imgopt_s3

import (
	"context"
	"fmt"
	"mime/multipart"
	"net/url"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type BucketBasis struct {
	Config  *aws.Config
	Client  *s3.Client
	Manager *transfermanager.Client
}

func (bb *BucketBasis) GetConfig() (*aws.Config, error) {
	if bb.Config != nil {
		return bb.Config, nil
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
	if err != nil {
		bb.Config = &cfg
	}
	return &cfg, err
}

func (bb *BucketBasis) GetClient() (*s3.Client, error) {
	if bb.Client != nil {
		return bb.Client, nil
	}

	cfg, err := bb.GetConfig()
	if err != nil {
		return nil, err
	}
	client := s3.NewFromConfig(*cfg)
	bb.Client = client
	return client, err
}

func (bb *BucketBasis) UploadFile(
	ctx context.Context,
	uploaderUuid,
	path string,
	file multipart.File,
	contentType string) (string, *s3.PutObjectOutput, error) {
	client, err := bb.GetClient()
	if err != nil {
		return "", nil, err
	}

	output, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(os.Getenv("S3_BUCKET")),
		Key:         aws.String(bb.GetFilePath(uploaderUuid, path)),
		Body:        file,
		ContentType: aws.String(contentType),
	})

	return bb.GetFileUrl(uploaderUuid, path), output, err
}

func (bb *BucketBasis) GetFilePath(uploaderUuid string, path string) string {
	return fmt.Sprintf("%v/%v/%v",
		os.Getenv("PROJECT_NAME"),
		uploaderUuid,
		path)
}

func (bb *BucketBasis) GetFileUrl(uploaderUuid string, p string) string {
	escapedPath := url.PathEscape(p)

	return fmt.Sprintf("%v/%v/%v",
		os.Getenv("S3_ENDPOINT_URL"),
		os.Getenv("S3_BUCKET"),
		bb.GetFilePath(uploaderUuid, escapedPath),
	)
}

func (bb *BucketBasis) IsAvailable() error {
	client, err := bb.GetClient()
	if err != nil {
		return err
	}

	_, err = client.HeadBucket(context.TODO(), &s3.HeadBucketInput{
		Bucket: aws.String(os.Getenv("S3_BUCKET")),
	})
	if err != nil {
		return err
	}

	return nil
}
