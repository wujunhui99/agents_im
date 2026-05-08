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

type CreateUploadIntentLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *usersvc.ServiceContext
}

func NewCreateUploadIntentLogic(ctx context.Context, svcCtx *usersvc.ServiceContext) *CreateUploadIntentLogic {
	return &CreateUploadIntentLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreateUploadIntentLogic) CreateUploadIntent(req *types.CreateMediaUploadReq) (*types.CreateMediaUploadResp, error) {
	userID, err := ctxuser.UserID(l.ctx)
	if err != nil {
		return nil, err
	}
	if l.svcCtx.MediaLogic == nil {
		return nil, apperror.Internal("media logic is not configured")
	}
	result, err := l.svcCtx.MediaLogic.CreateUploadIntent(l.ctx, business.CreateMediaUploadIntentRequest{
		OwnerUserID: userID,
		Purpose:     req.Purpose,
		Filename:    req.Filename,
		ContentType: req.ContentType,
		SizeBytes:   req.SizeBytes,
		SHA256:      req.SHA256,
		Width:       req.Width,
		Height:      req.Height,
	})
	if err != nil {
		return nil, err
	}
	return &types.CreateMediaUploadResp{
		Code:    string(apperror.CodeOK),
		Message: "ok",
		Data: types.CreateMediaUploadData{
			MediaID:   result.MediaID,
			ObjectKey: result.ObjectKey,
			UploadURL: result.UploadURL,
			ExpiresAt: result.ExpiresAt,
		},
	}, nil
}
