package storage

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/rs/zerolog/log"
)

type S3Storage struct {
	svc    *s3.Client
	bucket string
}

func (s *S3Storage) SetWithDeleteAfter(key string, value string, deleteAfter time.Duration) error {
	_, err := s.svc.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket: &s.bucket, Key: aws.String(key), Body: bytes.NewReader([]byte(value)),
		Metadata: map[string]string{"expires-at": strconv.FormatInt(time.Now().Add(deleteAfter).Unix(), 10)},
	})
	return err
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
		log.Fatal().Err(err).Msg("Failed to load SDK config")
	}

	svc := s3.NewFromConfig(awscfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String("http://" + host + ":" + strconv.Itoa(port))
		o.UsePathStyle = true
	})

	// Check if connection is established
	_, err = svc.ListBuckets(context.Background(), &s3.ListBucketsInput{})
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to S3")
	}

	// Create bucket if not exists
	_, err = svc.CreateBucket(context.Background(), &s3.CreateBucketInput{
		Bucket: &bucket,
	})
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create bucket")
	}

	return &S3Storage{svc: svc, bucket: bucket}
}

var ErrNotFound = errors.New("not found")

var _ Storage = (*S3Storage)(nil)

func (s *S3Storage) Set(key string, value string, skip_expiration bool) error {
	ctx := context.Background() // TODO: Add timeout control

	_, err := s.svc.PutObject(ctx, &s3.PutObjectInput{
		Bucket: &s.bucket,
		Key:    aws.String(key),
		Body:   bytes.NewReader([]byte(value)),
	})

	return err
}

func (s *S3Storage) Get(key string, skip_expiration bool) (string, error) {
	var nsk *types.NoSuchKey

	ctx := context.Background() // TODO: Add timeout control
	head, err := s.svc.HeadObject(ctx, &s3.HeadObjectInput{Bucket: &s.bucket, Key: aws.String(key)})
	var notFound *types.NotFound
	if errors.As(err, &notFound) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	if raw := head.Metadata["expires-at"]; raw != "" {
		expiresAt, parseErr := strconv.ParseInt(raw, 10, 64)
		if parseErr != nil {
			return "", parseErr
		}
		if time.Now().Unix() >= expiresAt {
			_, _ = s.svc.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: &s.bucket, Key: aws.String(key)})
			return "", ErrNotFound
		}
	}

	object, err := s.svc.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &s.bucket,
		Key:    aws.String(key),
	})
	if errors.As(err, &nsk) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	defer object.Body.Close()
	data, err := io.ReadAll(object.Body)
	return string(data), err
}

func (s *S3Storage) Close() error {
	return nil
}
