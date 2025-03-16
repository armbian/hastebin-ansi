package storage

import (
	"bytes"
	"context"
	"errors"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/sirupsen/logrus"
)

type S3Storage struct {
	svc        *s3.Client
	downloader *manager.Downloader
	uploader   *manager.Uploader
	bucket     string
}

func NewS3Storage(host string, port int, username string, password string, region string, bucket string) *S3Storage {
	creds := credentials.NewStaticCredentialsProvider(username, password, "")
	awscfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion(region),
		config.WithCredentialsProvider(creds),
		config.WithRetryer(func() aws.Retryer {
			return retry.AddWithMaxAttempts(retry.NewStandard(), 3)
		}),
	)

	if err != nil {
		logrus.Fatalf("unable to load SDK config, %v", err)
	}

	svc := s3.NewFromConfig(awscfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String("http://" + host + ":" + strconv.Itoa(port))
		o.UsePathStyle = true
	})

	downloader := manager.NewDownloader(svc)
	uploader := manager.NewUploader(svc)

	// Check if connection is established
	_, err = svc.ListBuckets(context.Background(), &s3.ListBucketsInput{})
	if err != nil {
		logrus.Fatalf("Failed to connect to S3: %v", err)
	}

	// Create bucket if not exists
	_, err = svc.CreateBucket(context.Background(), &s3.CreateBucketInput{
		Bucket: &bucket,
	})
	if err != nil {
		logrus.Fatalf("Failed to create bucket: %v", err)
	}

	return &S3Storage{svc: svc, downloader: downloader, uploader: uploader, bucket: bucket}
}

var ErrNotFound = errors.New("not found")

var _ Storage = (*S3Storage)(nil)

func (s *S3Storage) Set(key string, value string, skip_expiration bool) error {
	ctx := context.Background() // TODO: Add timeout control

	_, err := s.uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket: &s.bucket,
		Key:    aws.String(key),
		Body:   bytes.NewReader([]byte(value)),
	})

	return err
}

func (s *S3Storage) Get(key string, skip_expiration bool) (string, error) {
	var nsk *types.NoSuchKey

	ctx := context.Background() // TODO: Add timeout control

	buf := manager.NewWriteAtBuffer([]byte{})
	_, err := s.downloader.Download(ctx, buf, &s3.GetObjectInput{
		Bucket: &s.bucket,
		Key:    aws.String(key),
	})
	if errors.As(err, &nsk) {
		return "", ErrNotFound
	}

	return string(buf.Bytes()), err
}

func (s *S3Storage) Close() error {
	return nil
}
