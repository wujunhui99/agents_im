package svc

import (
	"context"

	"github.com/wujunhui99/agents_im/service/media/rpc/mediaclient"
)

// mediaRPCMessageValidator 把消息附件校验落到 media-rpc 上（#533）：image/file 引用的
// 存在/归属/ready/类型/大小校验由 media-rpc 拥有，msg 域不再直读 media_objects。
// 实现 internal/logic.MessageMediaValidator（msg-rpc 与 monolith messagesvc 共用同一接口）。
type mediaRPCMessageValidator struct {
	media mediaclient.Media
}

func newMediaRPCMessageValidator(media mediaclient.Media) *mediaRPCMessageValidator {
	return &mediaRPCMessageValidator{media: media}
}

func (v *mediaRPCMessageValidator) ValidateMessageMedia(ctx context.Context, ownerUserID, contentType, content string) error {
	_, err := v.media.ValidateMessageMedia(ctx, &mediaclient.ValidateMessageMediaRequest{
		OwnerUserId: ownerUserID,
		ContentType: contentType,
		Content:     content,
	})
	return err
}
