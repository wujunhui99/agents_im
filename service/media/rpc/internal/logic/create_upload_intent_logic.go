package logic

import (
	"context"

	business "github.com/wujunhui99/agents_im/service/media/core"
	"github.com/wujunhui99/agents_im/common/share/rpcerror"
	"github.com/wujunhui99/agents_im/service/media/rpc/internal/svc"
	"github.com/wujunhui99/agents_im/service/media/rpc/media"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateUploadIntentLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateUploadIntentLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateUploadIntentLogic {
	return &CreateUploadIntentLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CreateUploadIntentLogic) CreateUploadIntent(in *media.CreateUploadIntentRequest) (*media.CreateUploadIntentResponse, error) {
	resp, err := l.svcCtx.MediaLogic.CreateUploadIntent(l.ctx, business.CreateMediaUploadIntentRequest{
		OwnerUserID: in.GetOwnerUserId(),
		Purpose:     in.GetPurpose(),
		Filename:    in.GetFilename(),
		ContentType: in.GetContentType(),
		SizeBytes:   in.GetSizeBytes(),
		SHA256:      in.GetSha256(),
		Width:       in.GetWidth(),
		Height:      in.GetHeight(),
	})
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	return &media.CreateUploadIntentResponse{
		MediaId:   resp.MediaID,
		ObjectKey: resp.ObjectKey,
		UploadUrl: resp.UploadURL,
		ExpiresAt: resp.ExpiresAt,
	}, nil
}
