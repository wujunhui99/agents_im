package media

import (
	"context"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	business "github.com/wujunhui99/agents_im/internal/logic"
	usersvc "github.com/wujunhui99/agents_im/internal/servicecontext/user"
	"github.com/wujunhui99/agents_im/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetAvatarLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *usersvc.ServiceContext
}

func NewGetAvatarLogic(ctx context.Context, svcCtx *usersvc.ServiceContext) *GetAvatarLogic {
	return &GetAvatarLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetAvatarLogic) GetAvatar(req *types.GetMediaDownloadURLReq) (business.GetMediaDownloadURLResponse, error) {
	if l.svcCtx.MediaLogic == nil {
		return business.GetMediaDownloadURLResponse{}, apperror.Internal("media logic is not configured")
	}
	return l.svcCtx.MediaLogic.GetAvatarDisplayURL(l.ctx, req.MediaID)
}
