package svc

import (
	"context"

	"github.com/wujunhui99/agents_im/service/media/rpc/mediaclient"
)

// mediaRPCAvatarValidator 把 AvatarValidator 接口落到 media-rpc 上：头像 media 的存在/归属/
// ready/类型/大小校验由 media-rpc 拥有（#533），user-rpc 不再直读 media_objects。
type mediaRPCAvatarValidator struct {
	media mediaclient.Media
}

func newMediaRPCAvatarValidator(media mediaclient.Media) *mediaRPCAvatarValidator {
	return &mediaRPCAvatarValidator{media: media}
}

func (v *mediaRPCAvatarValidator) ValidateAvatarMedia(ctx context.Context, ownerUserID string, mediaID string) error {
	_, err := v.media.ValidateAvatarMedia(ctx, &mediaclient.ValidateAvatarMediaRequest{
		OwnerUserId: ownerUserID,
		MediaId:     mediaID,
	})
	return err
}
