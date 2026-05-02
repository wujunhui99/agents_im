package repository

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/wujunhui99/agents_im/internal/apperror"
	"github.com/wujunhui99/agents_im/internal/model"
)

type MemoryMediaRepository struct {
	mu        sync.RWMutex
	nextID    uint64
	byID      map[string]model.MediaObject
	objectKey map[string]string
	now       func() time.Time
}

func NewMemoryMediaRepository() *MemoryMediaRepository {
	return &MemoryMediaRepository{
		byID:      make(map[string]model.MediaObject),
		objectKey: make(map[string]string),
		now:       time.Now,
	}
}

func (r *MemoryMediaRepository) CreateMediaObject(_ context.Context, media model.MediaObject) (model.MediaObject, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if strings.TrimSpace(media.MediaID) == "" {
		r.nextID++
		media.MediaID = fmt.Sprintf("med_%06d", r.nextID)
	}
	if _, exists := r.byID[media.MediaID]; exists {
		return model.MediaObject{}, apperror.AlreadyExists("media object already exists")
	}
	if existingID := r.objectKey[media.ObjectKey]; existingID != "" {
		return model.MediaObject{}, apperror.AlreadyExists("media object key already exists")
	}
	if _, ok := model.NormalizeMediaPurpose(string(media.Purpose)); !ok {
		return model.MediaObject{}, apperror.InvalidArgument("media purpose is invalid")
	}
	if _, ok := model.NormalizeMediaStatus(string(media.Status)); !ok {
		return model.MediaObject{}, apperror.InvalidArgument("media status is invalid")
	}
	now := r.now().UTC()
	media.CreatedAt = now
	media.UpdatedAt = now
	r.byID[media.MediaID] = media.Clone()
	r.objectKey[media.ObjectKey] = media.MediaID
	return media.Clone(), nil
}

func (r *MemoryMediaRepository) GetMediaObject(_ context.Context, mediaID string) (model.MediaObject, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	media, ok := r.byID[mediaID]
	if !ok {
		return model.MediaObject{}, apperror.NotFound("media object not found")
	}
	return media.Clone(), nil
}

func (r *MemoryMediaRepository) UpdateMediaStatus(_ context.Context, mediaID string, status model.MediaStatus) (model.MediaObject, error) {
	if _, ok := model.NormalizeMediaStatus(string(status)); !ok {
		return model.MediaObject{}, apperror.InvalidArgument("media status is invalid")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	media, ok := r.byID[mediaID]
	if !ok {
		return model.MediaObject{}, apperror.NotFound("media object not found")
	}
	media.Status = status
	media.UpdatedAt = r.now().UTC()
	r.byID[mediaID] = media.Clone()
	return media.Clone(), nil
}

func (r *MemoryMediaRepository) ListMediaForOwner(ownerUserID string) []model.MediaObject {
	r.mu.RLock()
	defer r.mu.RUnlock()

	items := make([]model.MediaObject, 0)
	for _, media := range r.byID {
		if media.OwnerUserID == ownerUserID {
			items = append(items, media.Clone())
		}
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].CreatedAt.Before(items[j].CreatedAt)
	})
	return items
}
