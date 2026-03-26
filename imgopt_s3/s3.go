package imgopt_s3

import (
	"context"
	"io"
	"log"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type S3Actions struct {
	Config  *aws.Config
	Client  *s3.Client
	Manager *transfermanager.Client
}

func NewS3Action() *S3Actions {
	s3Actions := &S3Actions{}
	err := s3Actions.Init()
	if err != nil {
		log.Printf("WARNING: Could not initialize S3: %v", err)
	}
	return s3Actions
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

func (s *S3Actions) DownloadFile(ctx context.Context, bucket, key, destFilepath string) (*os.File, error) {
	client := s.Client

	result, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}
	defer result.Body.Close()

	file, err := os.Create(destFilepath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	body, err := io.ReadAll(result.Body)
	if err != nil {
		return nil, err
	}

	_, err = file.Write(body)
	if err != nil {
		return nil, err
	}

	return file, nil
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
