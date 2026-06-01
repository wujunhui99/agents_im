package repository

import (
	"context"

	"github.com/wujunhui99/agents_im/common/share/model"
)

type MediaRepository interface {
	CreateMediaObject(ctx context.Context, media model.MediaObject) (model.MediaObject, error)
	GetMediaObject(ctx context.Context, mediaID string) (model.MediaObject, error)
	UpdateMediaStatus(ctx context.Context, mediaID string, status model.MediaStatus) (model.MediaObject, error)
}
