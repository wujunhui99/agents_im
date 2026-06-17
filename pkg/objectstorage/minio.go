package objectstorage

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
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
	ExternalUseSSL   *bool
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
	externalUseSSL := cfg.UseSSL
	if cfg.ExternalUseSSL != nil {
		externalUseSSL = *cfg.ExternalUseSSL
	}
	if externalEndpoint := strings.TrimSpace(cfg.ExternalEndpoint); externalEndpoint != "" && (externalEndpoint != endpoint || externalUseSSL != cfg.UseSSL) {
		presignClient, err = minio.New(externalEndpoint, &minio.Options{
			Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
			Secure: externalUseSSL,
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

// PresignPutWithChecksum signs a PUT binding Content-Type and x-amz-checksum-sha256
// into the SigV4 signature (via PresignHeader). The client must replay both headers
// verbatim; OSS then computes the body SHA-256 and rejects a mismatch (BadDigest),
// so byte integrity is enforced by OSS, not media (EPIC #527 §3).
func (s *MinIOStore) PresignPutWithChecksum(ctx context.Context, objectKey, contentType string, sizeBytes int64, sha256Hex string, expires time.Duration) (string, error) {
	if sizeBytes <= 0 {
		return "", errors.New("object size must be positive")
	}
	if strings.TrimSpace(contentType) == "" {
		return "", errors.New("content type is required")
	}
	checksum, err := sha256HexToBase64(sha256Hex)
	if err != nil {
		return "", err
	}
	headers := http.Header{}
	headers.Set("Content-Type", contentType)
	headers.Set("x-amz-checksum-sha256", checksum)
	u, err := s.presignClient.PresignHeader(ctx, http.MethodPut, s.bucket, objectKey, expires, url.Values{}, headers)
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
	// Checksum: true sets x-amz-checksum-mode=ENABLED so HeadObject returns the
	// server-side SHA-256 (EPIC #527 §3 gating ③：取回 OSS 已校验的 ChecksumSHA256)。
	info, err := s.client.StatObject(ctx, s.bucket, objectKey, minio.StatObjectOptions{Checksum: true})
	if err != nil {
		if minio.ToErrorResponse(err).Code == "NoSuchKey" || minio.ToErrorResponse(err).StatusCode == 404 {
			return ObjectInfo{}, ErrObjectNotFound
		}
		return ObjectInfo{}, err
	}
	return ObjectInfo{
		ObjectKey:      objectKey,
		ETag:           info.ETag,
		ContentType:    info.ContentType,
		SizeBytes:      info.Size,
		LastModified:   info.LastModified,
		ChecksumSHA256: info.ChecksumSHA256,
	}, nil
}

// CopyObject performs a server-side copy (no external traffic) — used to rename
// tmp/{upload_id}/{sha256} → agents_im/{sha256} after OSS verifies the checksum.
func (s *MinIOStore) CopyObject(ctx context.Context, srcKey, dstKey string) error {
	_, err := s.client.CopyObject(ctx,
		minio.CopyDestOptions{Bucket: s.bucket, Object: dstKey},
		minio.CopySrcOptions{Bucket: s.bucket, Object: srcKey},
	)
	if err != nil {
		if minio.ToErrorResponse(err).Code == "NoSuchKey" || minio.ToErrorResponse(err).StatusCode == 404 {
			return ErrObjectNotFound
		}
		return err
	}
	return nil
}

// RemoveObject deletes objectKey; S3/MinIO delete is idempotent, so removing a
// missing object returns nil and tmp-cleanup retries stay safe.
func (s *MinIOStore) RemoveObject(ctx context.Context, objectKey string) error {
	return s.client.RemoveObject(ctx, s.bucket, objectKey, minio.RemoveObjectOptions{})
}

// ListByPrefix lists objects under prefix recursively (tmp sweeper).
func (s *MinIOStore) ListByPrefix(ctx context.Context, prefix string) ([]ObjectInfo, error) {
	var out []ObjectInfo
	for obj := range s.client.ListObjects(ctx, s.bucket, minio.ListObjectsOptions{Prefix: prefix, Recursive: true}) {
		if obj.Err != nil {
			return nil, obj.Err
		}
		out = append(out, ObjectInfo{
			ObjectKey:    obj.Key,
			ETag:         obj.ETag,
			ContentType:  obj.ContentType,
			SizeBytes:    obj.Size,
			LastModified: obj.LastModified,
		})
	}
	return out, nil
}

// sha256HexToBase64 converts a lowercase 64-hex SHA-256 digest into the base64
// form the x-amz-checksum-sha256 header requires.
func sha256HexToBase64(sha256Hex string) (string, error) {
	raw, err := hex.DecodeString(strings.TrimSpace(sha256Hex))
	if err != nil || len(raw) != 32 {
		return "", fmt.Errorf("invalid sha256 hex digest")
	}
	return base64.StdEncoding.EncodeToString(raw), nil
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
