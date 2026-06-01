package logic

import (
	"context"

	business "github.com/wujunhui99/agents_im/service/media/core"
	"github.com/wujunhui99/agents_im/common/share/rpcerror"
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

func (l *GetDownloadURLLogic) GetDownloadURL(in *media.GetDownloadURLRequest) (*media.GetDownloadURLResponse, error) {
	resp, err := l.svcCtx.MediaLogic.GetDownloadURL(l.ctx, business.GetMediaDownloadURLRequest{
		OwnerUserID:     in.GetOwnerUserId(),
		RequesterUserID: in.GetRequesterUserId(),
		MediaID:         in.GetMediaId(),
	})
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	return &media.GetDownloadURLResponse{
		MediaId:     resp.MediaID,
		DownloadUrl: resp.DownloadURL,
		ExpiresAt:   resp.ExpiresAt,
	}, nil
}
