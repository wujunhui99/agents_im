package objectstorage

import (
	"context"
	"errors"
	"time"
)

var ErrObjectNotFound = errors.New("object storage: object not found")

type ObjectInfo struct {
	ObjectKey    string
	ETag         string
	ContentType  string
	SizeBytes    int64
	LastModified time.Time
}

type ObjectStore interface {
	PresignPut(ctx context.Context, objectKey, contentType string, sizeBytes int64, expires time.Duration) (string, error)
	PresignGet(ctx context.Context, objectKey string, expires time.Duration) (string, error)
	StatObject(ctx context.Context, objectKey string) (ObjectInfo, error)
	EnsureBucket(ctx context.Context) error
}
