package logic

import (
	"context"

	"github.com/wujunhui99/agents_im/service/media/rpc/internal/svc"
	"github.com/wujunhui99/agents_im/service/media/rpc/media"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetAvatarDisplayURLLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetAvatarDisplayURLLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetAvatarDisplayURLLogic {
	return &GetAvatarDisplayURLLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetAvatarDisplayURLLogic) GetAvatarDisplayURL(in *media.GetAvatarDisplayURLRequest) (*media.GetDownloadURLResponse, error) {
	resp, err := l.svcCtx.MediaLogic.GetAvatarDisplayURL(l.ctx, in.GetMediaId())
	if err != nil {
		return nil, err
	}
	return &media.GetDownloadURLResponse{
		MediaId:     resp.MediaID,
		DownloadUrl: resp.DownloadURL,
		ExpiresAt:   resp.ExpiresAt,
	}, nil
}
