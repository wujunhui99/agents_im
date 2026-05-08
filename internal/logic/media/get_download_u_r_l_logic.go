package media

import (
	"context"

	"github.com/wujunhui99/agents_im/internal/apperror"
	"github.com/wujunhui99/agents_im/internal/ctxuser"
	business "github.com/wujunhui99/agents_im/internal/logic"
	usersvc "github.com/wujunhui99/agents_im/internal/servicecontext/user"
	"github.com/wujunhui99/agents_im/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetDownloadURLLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *usersvc.ServiceContext
}

func NewGetDownloadURLLogic(ctx context.Context, svcCtx *usersvc.ServiceContext) *GetDownloadURLLogic {
	return &GetDownloadURLLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetDownloadURLLogic) GetDownloadURL(req *types.GetMediaDownloadURLReq) (*types.GetMediaDownloadURLResp, error) {
	userID, err := ctxuser.UserID(l.ctx)
	if err != nil {
		return nil, err
	}
	if l.svcCtx.MediaLogic == nil {
		return nil, apperror.Internal("media logic is not configured")
	}
	result, err := l.svcCtx.MediaLogic.GetDownloadURL(l.ctx, business.GetMediaDownloadURLRequest{
		OwnerUserID: userID,
		MediaID:     req.MediaID,
	})
	if err != nil {
		return nil, err
	}
	return &types.GetMediaDownloadURLResp{
		Code:    string(apperror.CodeOK),
		Message: "ok",
		Data: types.GetMediaDownloadURLData{
			MediaID:     result.MediaID,
			DownloadURL: result.DownloadURL,
			ExpiresAt:   result.ExpiresAt,
		},
	}, nil
}
