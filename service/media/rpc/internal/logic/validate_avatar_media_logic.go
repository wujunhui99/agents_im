package logic

import (
	"context"

	"github.com/wujunhui99/agents_im/common/share/rpcerror"
	"github.com/wujunhui99/agents_im/service/media/rpc/internal/svc"
	"github.com/wujunhui99/agents_im/service/media/rpc/media"

	"github.com/zeromicro/go-zero/core/logx"
)

type ValidateAvatarMediaLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewValidateAvatarMediaLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ValidateAvatarMediaLogic {
	return &ValidateAvatarMediaLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ValidateAvatarMedia 校验 media_id 是 owner 拥有的、ready 的头像对象，且类型/大小合法
// （#533，取代 internal/mediavalidate.AvatarValidator）。成功返回空响应，失败映射成 gRPC status。
func (l *ValidateAvatarMediaLogic) ValidateAvatarMedia(in *media.ValidateAvatarMediaRequest) (*media.ValidateMediaResponse, error) {
	mediaID, err := parseMediaID(in.GetMediaId())
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	obj, err := mediaForOwner(l.ctx, l.svcCtx, in.GetOwnerUserId(), mediaID)
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	if err := validateAvatarMediaObject(obj); err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	return &media.ValidateMediaResponse{}, nil
}
