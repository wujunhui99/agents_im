package logic

import (
	"context"
	"time"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/pkg/rpcerror"
	"github.com/wujunhui99/agents_im/service/resource/rpc/internal/media/model"
	"github.com/wujunhui99/agents_im/service/resource/rpc/internal/media/svc"
	"github.com/wujunhui99/agents_im_protocol/media/v1"

	"github.com/zeromicro/go-zero/core/logx"
)

// maxBatchDownloadURLs 限制单次批量签发数量：一次消息拉取的附件数远小于此值。
const maxBatchDownloadURLs = 200

type GetDownloadURLsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetDownloadURLsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetDownloadURLsLogic {
	return &GetDownloadURLsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetDownloadURLs 批量签发 presigned GET。单个 media 不存在/未 ready/签名失败时
// 直接跳过（调用方按缺失项回退到单条下载端点），不让一条坏附件拖垮整页历史。
func (l *GetDownloadURLsLogic) GetDownloadURLs(in *media.GetDownloadURLsRequest) (*media.GetDownloadURLsResponse, error) {
	if l.svcCtx.Store == nil {
		return nil, rpcerror.ToStatus(apperror.Internal("object store is not configured"))
	}
	seen := make(map[string]struct{}, len(in.GetMediaIds()))
	out := &media.GetDownloadURLsResponse{Urls: make([]*media.GetDownloadURLResponse, 0, len(in.GetMediaIds()))}
	expiresAt := time.Now().UTC().Add(downloadURLTTL)
	for _, raw := range in.GetMediaIds() {
		if len(out.Urls) >= maxBatchDownloadURLs {
			break
		}
		mediaID, err := parseMediaID(raw)
		if err != nil {
			continue
		}
		key := formatMediaID(mediaID)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		obj, err := mediaByID(l.ctx, l.svcCtx, mediaID)
		if err != nil || obj.Status != model.MediaStatusReady {
			continue
		}
		downloadURL, err := l.svcCtx.Store.PresignGet(l.ctx, obj.ObjectKey, downloadURLTTL)
		if err != nil {
			l.Errorf("presign media %s: %v", key, err)
			continue
		}
		out.Urls = append(out.Urls, &media.GetDownloadURLResponse{
			MediaId:     key,
			DownloadUrl: downloadURL,
			ExpiresAt:   expiresAt.UnixMilli(),
		})
	}
	return out, nil
}
