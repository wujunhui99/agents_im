package logic

import (
	"context"

	"github.com/wujunhui99/agents_im/pkg/rpcerror"
	"github.com/wujunhui99/agents_im/service/media/rpc/internal/svc"
	"github.com/wujunhui99/agents_im/service/media/rpc/media"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetMediaLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetMediaLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetMediaLogic {
	return &GetMediaLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

// GetMedia 返回 media 元数据（uploader/status/object_key 等），供 media-api(BFF) 做下载授权时
// 判 uploader 快速放行（EPIC #527 §4，鉴权编排在 media-api，rpc 不做跨域调用）。
func (l *GetMediaLogic) GetMedia(in *media.GetMediaRequest) (*media.MediaObject, error) {
	mediaID, err := parseMediaID(in.GetMediaId())
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	obj, err := mediaByID(l.ctx, l.svcCtx, mediaID)
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	return toPBMediaObject(obj), nil
}
