package objectstorage

import (
	"context"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

type MemoryStore struct {
	mu      sync.RWMutex
	objects map[string]ObjectInfo
	now     func() time.Time
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		objects: make(map[string]ObjectInfo),
		now:     time.Now,
	}
}

func (s *MemoryStore) PresignPut(_ context.Context, objectKey, contentType string, sizeBytes int64, expires time.Duration) (string, error) {
	return memoryURL("put", objectKey, contentType, sizeBytes, s.now().Add(expires)), nil
}

func (s *MemoryStore) PresignPutWithChecksum(_ context.Context, objectKey, contentType string, sizeBytes int64, sha256Hex string, expires time.Duration) (string, error) {
	u := memoryURL("put", objectKey, contentType, sizeBytes, s.now().Add(expires))
	return u + "&checksumSha256=" + url.QueryEscape(sha256Hex), nil
}

func (s *MemoryStore) PresignGet(_ context.Context, objectKey string, expires time.Duration) (string, error) {
	return memoryURL("get", objectKey, "", 0, s.now().Add(expires)), nil
}

func (s *MemoryStore) StatObject(_ context.Context, objectKey string) (ObjectInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	info, ok := s.objects[objectKey]
	if !ok {
		return ObjectInfo{}, ErrObjectNotFound
	}
	return info, nil
}

func (s *MemoryStore) CopyObject(_ context.Context, srcKey, dstKey string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	src, ok := s.objects[strings.TrimSpace(srcKey)]
	if !ok {
		return ErrObjectNotFound
	}
	dst := src
	dst.ObjectKey = strings.TrimSpace(dstKey)
	s.objects[dst.ObjectKey] = dst
	return nil
}

func (s *MemoryStore) RemoveObject(_ context.Context, objectKey string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.objects, strings.TrimSpace(objectKey))
	return nil
}

func (s *MemoryStore) ListByPrefix(_ context.Context, prefix string) ([]ObjectInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var out []ObjectInfo
	for key, info := range s.objects {
		if strings.HasPrefix(key, prefix) {
			out = append(out, info)
		}
	}
	return out, nil
}

func (s *MemoryStore) EnsureBucket(context.Context) error {
	return nil
}

func (s *MemoryStore) PutObjectInfo(info ObjectInfo) {
	s.mu.Lock()
	defer s.mu.Unlock()

	info.ObjectKey = strings.TrimSpace(info.ObjectKey)
	if info.LastModified.IsZero() {
		info.LastModified = s.now().UTC()
	}
	s.objects[info.ObjectKey] = info
}

func memoryURL(kind, objectKey, contentType string, sizeBytes int64, expiresAt time.Time) string {
	values := url.Values{}
	if contentType != "" {
		values.Set("contentType", contentType)
	}
	if sizeBytes > 0 {
		values.Set("sizeBytes", formatInt64(sizeBytes))
	}
	values.Set("expiresAt", formatInt64(expiresAt.UTC().UnixMilli()))
	return "memory://" + kind + "/" + url.PathEscape(objectKey) + "?" + values.Encode()
}

func formatInt64(value int64) string {
	return strconv.FormatInt(value, 10)
}
