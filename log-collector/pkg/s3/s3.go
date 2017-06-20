package s3

import (
	"io"
	"path/filepath"

	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

type Config struct {
	AccessKeyId     string
	AccessKeySecret string
	BucketName      string
	BucketPrefix    string
	Region          string
}

type S3 struct {
	client       *s3manager.Uploader
	bucketName   string
	bucketPrefix string
}

func New(config *Config) (*S3, error) {
	creds := credentials.NewStaticCredentials(config.AccessKeyId, config.AccessKeySecret, "")
	var err error
	_, err = creds.Get()
	if err != nil {
		return nil, err
	}

	cfg := aws.NewConfig().WithRegion(config.Region).WithCredentials(creds).WithEndpoint("s3.amazonaws.com").WithS3ForcePathStyle(true)
	svc := s3.New(session.New(), cfg)

	uploader := s3manager.NewUploaderWithClient(svc, func(u *s3manager.Uploader) {
		u.LeavePartsOnError = true
		u.PartSize = 10 * 1024 * 1024
	})
	return &S3{
		client:       uploader,
		bucketName:   config.BucketName,
		bucketPrefix: config.BucketPrefix,
	}, nil
}

func (s *S3) Put(f io.ReadSeeker, dst string) (string, error) {
	path := filepath.Join(s.bucketPrefix, dst)
	params := &s3manager.UploadInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(path),

		Body: f,
	}

	resp, err := s.client.Upload(params)
	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			return "", fmt.Errorf("Code: %s Message: %s Orig: %v", awsErr.Code(), awsErr.Message(), awsErr.OrigErr())
		}
		return "", err
	}

	return resp.Location, nil
}
