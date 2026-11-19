package svc

import (
	"context"

	"github.com/wujunhui99/agents_im/service/resource/rpc/mediaclient"
)

// MediaURL 是批量为消息附件签发的短 TTL presigned GET。
type MediaURL struct {
	URL       string
	ExpiresAt int64 // unix 毫秒
}

// MediaURLSigner 是「按 media_id 批量签发下载 URL」的跨域端口（resource-rpc/media）。
// 读取路径在属主域内一次签齐附件 URL，客户端直连对象存储，不再逐条做下载鉴权。
type MediaURLSigner interface {
	SignDownloadURLs(ctx context.Context, mediaIDs []string) (map[string]MediaURL, error)
}

type mediaURLSigner struct {
	media mediaclient.Media
}

func newMediaURLSigner(media mediaclient.Media) *mediaURLSigner {
	return &mediaURLSigner{media: media}
}

func (s *mediaURLSigner) SignDownloadURLs(ctx context.Context, mediaIDs []string) (map[string]MediaURL, error) {
	if len(mediaIDs) == 0 {
		return nil, nil
	}
	resp, err := s.media.GetDownloadURLs(ctx, &mediaclient.GetDownloadURLsRequest{MediaIds: mediaIDs})
	if err != nil {
		return nil, err
	}
	out := make(map[string]MediaURL, len(resp.GetUrls()))
	for _, item := range resp.GetUrls() {
		out[item.GetMediaId()] = MediaURL{URL: item.GetDownloadUrl(), ExpiresAt: item.GetExpiresAt()}
	}
	return out, nil
}
