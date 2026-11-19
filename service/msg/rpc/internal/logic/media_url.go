package logic

import (
	"context"
	"encoding/json"

	"github.com/wujunhui99/agents_im/service/msg/rpc/internal/model"
	"github.com/wujunhui99/agents_im/service/msg/rpc/internal/svc"
	"github.com/zeromicro/go-zero/core/logx"

	msgpb "github.com/wujunhui99/agents_im_protocol/msg/v1"
)

// attachMediaURLs 为图片/文件消息批量签发 presigned GET（一次 RPC 签齐整页附件），
// 客户端拿到 media_url 后直连对象存储，不再逐条调下载鉴权端点。签发失败不影响拉取：
// 消息照常返回，media_url 为空时客户端回退到 /media/{id}/download-url。
func attachMediaURLs(ctx context.Context, signer svc.MediaURLSigner, rows []*model.Messages, out []*msgpb.Message) {
	if signer == nil || len(rows) == 0 {
		return
	}
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		if id := mediaIDFromMessage(row); id != "" {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return
	}
	urls, err := signer.SignDownloadURLs(ctx, ids)
	if err != nil {
		logx.WithContext(ctx).Errorf("sign media urls failed (messages still returned): %v", err)
		return
	}
	for i, row := range rows {
		if i >= len(out) {
			break
		}
		if mediaURL, ok := urls[mediaIDFromMessage(row)]; ok {
			out[i].MediaUrl = mediaURL.URL
			out[i].MediaUrlExpiresAt = mediaURL.ExpiresAt
		}
	}
}

// mediaIDFromMessage 从 image/file 消息的 content JSON 里取 mediaId。
func mediaIDFromMessage(m *model.Messages) string {
	if m == nil || (m.ContentType != model.ContentTypeImageValue && m.ContentType != model.ContentTypeFileValue) {
		return ""
	}
	var payload struct {
		MediaID    string `json:"mediaId"`
		MediaIDAlt string `json:"media_id"`
	}
	if err := json.Unmarshal([]byte(m.Content), &payload); err != nil {
		return ""
	}
	if payload.MediaID != "" {
		return payload.MediaID
	}
	return payload.MediaIDAlt
}
