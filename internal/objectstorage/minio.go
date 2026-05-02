package objectstorage

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type Config struct {
	Driver           string
	Endpoint         string
	ExternalEndpoint string
	Bucket           string
	Region           string
	UseSSL           bool
	AccessKeyID      string
	SecretAccessKey  string
}

type MinIOStore struct {
	client        *minio.Client
	presignClient *minio.Client
	bucket        string
}

func NewMinIOStore(cfg Config) (*MinIOStore, error) {
	endpoint := strings.TrimSpace(cfg.Endpoint)
	bucket := strings.TrimSpace(cfg.Bucket)
	accessKey := strings.TrimSpace(cfg.AccessKeyID)
	secretKey := strings.TrimSpace(cfg.SecretAccessKey)
	if endpoint == "" {
		return nil, errors.New("object storage endpoint is required")
	}
	if bucket == "" {
		return nil, errors.New("object storage bucket is required")
	}
	if accessKey == "" {
		return nil, errors.New("object storage access key is required")
	}
	if secretKey == "" {
		return nil, errors.New("object storage secret key is required")
	}

	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: cfg.UseSSL,
		Region: strings.TrimSpace(cfg.Region),
	})
	if err != nil {
		return nil, err
	}

	presignClient := client
	if externalEndpoint := strings.TrimSpace(cfg.ExternalEndpoint); externalEndpoint != "" && externalEndpoint != endpoint {
		presignClient, err = minio.New(externalEndpoint, &minio.Options{
			Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
			Secure: cfg.UseSSL,
			Region: strings.TrimSpace(cfg.Region),
		})
		if err != nil {
			return nil, err
		}
	}

	return &MinIOStore{client: client, presignClient: presignClient, bucket: bucket}, nil
}

func (s *MinIOStore) PresignPut(ctx context.Context, objectKey, contentType string, sizeBytes int64, expires time.Duration) (string, error) {
	if sizeBytes <= 0 {
		return "", errors.New("object size must be positive")
	}
	if strings.TrimSpace(contentType) == "" {
		return "", errors.New("content type is required")
	}
	u, err := s.presignClient.PresignedPutObject(ctx, s.bucket, objectKey, expires)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

func (s *MinIOStore) PresignGet(ctx context.Context, objectKey string, expires time.Duration) (string, error) {
	u, err := s.presignClient.PresignedGetObject(ctx, s.bucket, objectKey, expires, url.Values{})
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

func (s *MinIOStore) StatObject(ctx context.Context, objectKey string) (ObjectInfo, error) {
	info, err := s.client.StatObject(ctx, s.bucket, objectKey, minio.StatObjectOptions{})
	if err != nil {
		if minio.ToErrorResponse(err).Code == "NoSuchKey" || minio.ToErrorResponse(err).StatusCode == 404 {
			return ObjectInfo{}, ErrObjectNotFound
		}
		return ObjectInfo{}, err
	}
	return ObjectInfo{
		ObjectKey:    objectKey,
		ETag:         info.ETag,
		ContentType:  info.ContentType,
		SizeBytes:    info.Size,
		LastModified: info.LastModified,
	}, nil
}

func (s *MinIOStore) EnsureBucket(ctx context.Context) error {
	exists, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	if err := s.client.MakeBucket(ctx, s.bucket, minio.MakeBucketOptions{}); err != nil {
		return fmt.Errorf("create object storage bucket %q: %w", s.bucket, err)
	}
	return nil
}
