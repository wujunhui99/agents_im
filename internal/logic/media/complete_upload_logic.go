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

type CompleteUploadLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *usersvc.ServiceContext
}

func NewCompleteUploadLogic(ctx context.Context, svcCtx *usersvc.ServiceContext) *CompleteUploadLogic {
	return &CompleteUploadLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CompleteUploadLogic) CompleteUpload(req *types.CompleteMediaUploadReq) (*types.CompleteMediaUploadResp, error) {
	userID, err := ctxuser.UserID(l.ctx)
	if err != nil {
		return nil, err
	}
	if l.svcCtx.MediaLogic == nil {
		return nil, apperror.Internal("media logic is not configured")
	}
	result, err := l.svcCtx.MediaLogic.CompleteUpload(l.ctx, business.CompleteMediaUploadRequest{
		OwnerUserID: userID,
		MediaID:     req.MediaID,
	})
	if err != nil {
		return nil, err
	}
	return &types.CompleteMediaUploadResp{
		Code:    string(apperror.CodeOK),
		Message: "ok",
		Data:    types.CompleteMediaUploadData{Media: toMediaObject(result.Media)},
	}, nil
}
