package logic

import (
	"context"

	business "github.com/wujunhui99/agents_im/service/media/core"
	"github.com/wujunhui99/agents_im/common/share/rpcerror"
	"github.com/wujunhui99/agents_im/service/media/rpc/internal/svc"
	"github.com/wujunhui99/agents_im/service/media/rpc/media"

	"github.com/zeromicro/go-zero/core/logx"
)

type CompleteUploadLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCompleteUploadLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CompleteUploadLogic {
	return &CompleteUploadLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CompleteUploadLogic) CompleteUpload(in *media.CompleteUploadRequest) (*media.CompleteUploadResponse, error) {
	resp, err := l.svcCtx.MediaLogic.CompleteUpload(l.ctx, business.CompleteMediaUploadRequest{
		OwnerUserID: in.GetOwnerUserId(),
		MediaID:     in.GetMediaId(),
	})
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	return &media.CompleteUploadResponse{
		Media: mediaObjectToPB(resp.Media),
	}, nil
}

func mediaObjectToPB(m business.MediaObject) *media.MediaObject {
	return &media.MediaObject{
		MediaId:          m.MediaID,
		OwnerUserId:      m.OwnerUserID,
		Bucket:           m.Bucket,
		ObjectKey:        m.ObjectKey,
		Sha256:           m.SHA256,
		ContentType:      m.ContentType,
		SizeBytes:        m.SizeBytes,
		Width:            m.Width,
		Height:           m.Height,
		OriginalFilename: m.OriginalFilename,
		Purpose:          m.Purpose,
		Status:           m.Status,
		CreatedAt:        m.CreatedAt,
		UpdatedAt:        m.UpdatedAt,
	}
}
