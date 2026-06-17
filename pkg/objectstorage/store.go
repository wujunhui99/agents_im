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
	// ChecksumSHA256 is the OSS server-side SHA-256 checksum (base64) returned by
	// HeadObject when the object was uploaded with x-amz-checksum-sha256; empty when
	// the object carries no server-side checksum. media-rpc reads this as the OSS
	// verification verdict (EPIC #527 §3：校验是 OSS 的职责，media 不碰字节、不回算)。
	ChecksumSHA256 string
}

type ObjectStore interface {
	// PresignPut signs a plain PUT (no checksum enforcement). Retained for callers
	// that do not content-address; content-addressed uploads use PresignPutWithChecksum.
	PresignPut(ctx context.Context, objectKey, contentType string, sizeBytes int64, expires time.Duration) (string, error)
	// PresignPutWithChecksum signs a PUT that binds x-amz-checksum-sha256 into the
	// signature, so the client must send that exact header and OSS rejects bytes whose
	// SHA-256 does not match (EPIC #527 §3). sha256Hex is the lowercase 64-hex whole-file
	// digest; the store converts it to the base64 the header requires.
	PresignPutWithChecksum(ctx context.Context, objectKey, contentType string, sizeBytes int64, sha256Hex string, expires time.Duration) (string, error)
	PresignGet(ctx context.Context, objectKey string, expires time.Duration) (string, error)
	// StatObject HEADs the object and includes the server-side SHA-256 checksum in
	// ObjectInfo.ChecksumSHA256 when present.
	StatObject(ctx context.Context, objectKey string) (ObjectInfo, error)
	// CopyObject performs a server-side copy (no external traffic) from srcKey to
	// dstKey; used to rename tmp/{upload_id}/{sha256} → agents_im/{sha256}.
	CopyObject(ctx context.Context, srcKey, dstKey string) error
	// RemoveObject deletes objectKey. Removing a missing object is not an error
	// (idempotent), so tmp cleanup and copy/delete rename retries stay safe.
	RemoveObject(ctx context.Context, objectKey string) error
	// ListByPrefix lists objects under prefix (recursive); used by the tmp sweeper.
	ListByPrefix(ctx context.Context, prefix string) ([]ObjectInfo, error)
	EnsureBucket(ctx context.Context) error
}
