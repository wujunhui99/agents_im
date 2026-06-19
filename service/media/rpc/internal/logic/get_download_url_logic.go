package logic

import (
	"context"
	"time"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/pkg/rpcerror"
	"github.com/wujunhui99/agents_im/service/media/rpc/internal/model"
	"github.com/wujunhui99/agents_im/service/media/rpc/internal/svc"
	"github.com/wujunhui99/agents_im/service/media/rpc/media"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetDownloadURLLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetDownloadURLLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetDownloadURLLogic {
	return &GetDownloadURLLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetDownloadURL 纯签发整文件 presigned GET：跨域下载授权（链路校验 + 私聊好友 / 群成员）已上移
// media-api(BFF)，本 RPC 只校验 media 存在且 ready 后签 URL，不做 requester 鉴权（EPIC #527 §4，
// media-rpc 不发起跨域 rpc 调用，保持叶子）。
func (l *GetDownloadURLLogic) GetDownloadURL(in *media.GetDownloadURLRequest) (*media.GetDownloadURLResponse, error) {
	mediaID, err := parseMediaID(in.GetMediaId())
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	obj, err := mediaByID(l.ctx, l.svcCtx, mediaID)
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	if obj.Status != model.MediaStatusReady {
		return nil, rpcerror.ToStatus(apperror.InvalidArgument("media object is not ready"))
	}
	if l.svcCtx.Store == nil {
		return nil, rpcerror.ToStatus(apperror.Internal("object store is not configured"))
	}
	expiresAt := time.Now().UTC().Add(downloadURLTTL)
	downloadURL, err := l.svcCtx.Store.PresignGet(l.ctx, obj.ObjectKey, downloadURLTTL)
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	return &media.GetDownloadURLResponse{
		MediaId:     formatMediaID(obj.MediaId),
		DownloadUrl: downloadURL,
		ExpiresAt:   expiresAt.UnixMilli(),
	}, nil
}
