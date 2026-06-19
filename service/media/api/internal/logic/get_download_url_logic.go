package logic

import (
	"context"

	"github.com/wujunhui99/agents_im/service/media/api/internal/svc"
	"github.com/wujunhui99/agents_im/service/media/rpc/mediaclient"
)

// GetDownloadURL 是 media-api(BFF) 的下载入口：先聚合 msg/friends/groups 做下载授权
// （authorizeDownload，EPIC #527 §4），通过后调 media-rpc 纯签发 presigned GET。
// 返回的 error 为 apperror 或下游 gRPC status，由 handler 的 apiError 映射成 HTTP 码。
func GetDownloadURL(ctx context.Context, svcCtx *svc.ServiceContext, requester, mediaID, msgID string) (*mediaclient.GetDownloadURLResponse, error) {
	if err := authorizeDownload(ctx, requester, mediaID, msgID, svcCtx.MediaRPC, svcCtx.MsgRPC, svcCtx.FriendsRPC, svcCtx.GroupsRPC); err != nil {
		return nil, err
	}
	return svcCtx.MediaRPC.GetDownloadURL(ctx, &mediaclient.GetDownloadURLRequest{MediaId: mediaID})
}
