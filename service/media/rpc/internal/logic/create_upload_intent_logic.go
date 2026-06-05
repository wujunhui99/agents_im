package logic

import (
	"context"
	"time"

	"github.com/wujunhui99/agents_im/common/share/rpcerror"
	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/service/media/rpc/internal/model"
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
	owner, err := validateMediaIDComponent(in.GetOwnerUserId(), "owner_user_id")
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	input, err := validateUploadIntent(in)
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	if l.svcCtx.Store == nil {
		return nil, rpcerror.ToStatus(apperror.Internal("object store is not configured"))
	}
	bucket := l.svcCtx.Bucket
	if bucket == "" {
		return nil, rpcerror.ToStatus(apperror.Internal("object storage bucket is not configured"))
	}

	mediaID, err := newMediaID()
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	objectKey := mediaObjectKey(owner, mediaID, input.filename)
	expiresAt := time.Now().UTC().Add(uploadURLTTL)
	uploadURL, err := l.svcCtx.Store.PresignPut(l.ctx, objectKey, input.contentType, in.GetSizeBytes(), uploadURLTTL)
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}

	purposeDB, _ := purposeToDB(input.purpose)
	metadata, err := uploadMetadataJSON(input.sha256, in.GetWidth(), in.GetHeight())
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	created, err := l.svcCtx.MediaModel.CreateMediaObject(l.ctx, &model.MediaObjects{
		MediaId:          mediaID,
		OwnerAccountId:   owner,
		Bucket:           bucket,
		ObjectKey:        objectKey,
		OriginalFilename: input.filename,
		ContentType:      input.contentType,
		SizeBytes:        in.GetSizeBytes(),
		Purpose:          purposeDB,
		Status:           model.MediaStatusPending,
		Metadata:         metadata,
	})
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	return &media.CreateUploadIntentResponse{
		MediaId:   created.MediaId,
		ObjectKey: created.ObjectKey,
		UploadUrl: uploadURL,
		ExpiresAt: expiresAt.UnixMilli(),
	}, nil
}
