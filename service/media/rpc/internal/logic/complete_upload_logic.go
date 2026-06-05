package logic

import (
	"context"
	"errors"

	"github.com/wujunhui99/agents_im/common/share/rpcerror"
	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/pkg/objectstorage"
	"github.com/wujunhui99/agents_im/service/media/rpc/internal/model"
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
	obj, err := mediaForOwner(l.ctx, l.svcCtx, in.GetOwnerUserId(), in.GetMediaId())
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	if obj.Status != model.MediaStatusPending {
		return nil, rpcerror.ToStatus(apperror.InvalidArgument("media object is not pending"))
	}
	if l.svcCtx.Store == nil {
		return nil, rpcerror.ToStatus(apperror.Internal("object store is not configured"))
	}
	info, err := l.svcCtx.Store.StatObject(l.ctx, obj.ObjectKey)
	if err != nil {
		if errors.Is(err, objectstorage.ErrObjectNotFound) {
			return nil, rpcerror.ToStatus(apperror.NotFound("uploaded object not found"))
		}
		return nil, rpcerror.ToStatus(err)
	}
	if info.SizeBytes != obj.SizeBytes {
		return nil, rpcerror.ToStatus(apperror.InvalidArgument("uploaded object size does not match upload intent"))
	}
	if normalizeContentType(info.ContentType) != obj.ContentType {
		return nil, rpcerror.ToStatus(apperror.InvalidArgument("uploaded object content_type does not match upload intent"))
	}
	updated, err := l.svcCtx.MediaModel.UpdateStatus(l.ctx, obj.MediaId, model.MediaStatusReady)
	if err != nil {
		if err == model.ErrNotFound {
			return nil, rpcerror.ToStatus(apperror.NotFound("media object not found"))
		}
		return nil, rpcerror.ToStatus(err)
	}
	return &media.CompleteUploadResponse{
		Media: toPBMediaObject(updated),
	}, nil
}
