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

// maxObjectKeyAttempts bounds object_key 重生重试次数。RAND16 是 64-bit 随机，在
// {uploader_last4}/{yyyymmdd}/ 作用域内碰撞概率可忽略，几次足以兜底唯一约束冲突（EPIC #527 §1）。
const maxObjectKeyAttempts = 5

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

	mediaID, err := l.svcCtx.MediaIDGen.Next(0) // media HintBits=0（无单/群语义），hint 恒传 0。
	if err != nil {
		return nil, rpcerror.ToStatus(apperror.Internal("could not allocate media id"))
	}

	metadata, err := uploadMetadataJSON(input.sha256, in.GetWidth(), in.GetHeight())
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}

	created, err := l.createPendingObject(mediaID, owner, bucket, input, metadata, in.GetSizeBytes())
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}

	expiresAt := time.Now().UTC().Add(uploadURLTTL)
	uploadURL, err := l.svcCtx.Store.PresignPut(l.ctx, created.ObjectKey, input.contentType, in.GetSizeBytes(), uploadURLTTL)
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	return &media.CreateUploadIntentResponse{
		MediaId:   formatMediaID(created.MediaId),
		ObjectKey: created.ObjectKey,
		UploadUrl: uploadURL,
		ExpiresAt: expiresAt.UnixMilli(),
	}, nil
}

// createPendingObject 落 pending 行；object_key 唯一约束冲突时重生 RAND16 token 重试。
func (l *CreateUploadIntentLogic) createPendingObject(mediaID int64, owner, bucket string, input uploadIntentInput, metadata string, sizeBytes int64) (*model.MediaObjects, error) {
	purposeDB, _ := purposeToDB(input.purpose)
	var lastErr error
	for attempt := 0; attempt < maxObjectKeyAttempts; attempt++ {
		objectKey, err := mediaObjectKey(owner, input.contentType)
		if err != nil {
			return nil, err
		}
		created, err := l.svcCtx.MediaModel.CreateMediaObject(l.ctx, &model.MediaObjects{
			MediaId:          mediaID,
			UploaderId:       owner,
			Bucket:           bucket,
			ObjectKey:        objectKey,
			OriginalFilename: input.filename,
			ContentType:      input.contentType,
			SizeBytes:        sizeBytes,
			Purpose:          purposeDB,
			Status:           model.MediaStatusPending,
			Metadata:         metadata,
		})
		if err == nil {
			return created, nil
		}
		if !model.IsObjectKeyConflict(err) {
			return nil, err
		}
		lastErr = err
	}
	logx.WithContext(l.ctx).Errorf("media object_key collision unresolved after %d attempts: %v", maxObjectKeyAttempts, lastErr)
	return nil, apperror.Internal("could not allocate unique object key")
}
