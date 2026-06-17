package logic

import (
	"context"
	"errors"
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

// CreateUploadIntent 内容寻址上传第一步：校验入参 → 文件级秒传命中则零字节落 ready 行；未命中则
// 落 pending 行并签发带 x-amz-checksum-sha256 的 presigned PUT 到 tmp/{media_id}/{sha256}（EPIC #527 §3）。
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

	finalKey := finalObjectKey(input.sha256)

	// 文件级秒传：finalKey 已有 ready 行 → 字节已在 OSS，零传直接落新 ready 行（多行共享 object_key，
	// 021 已去唯一约束）。复用既有行的真实 size/content-type，确保 1 object : N 行字节口径一致。
	if existing, err := l.svcCtx.MediaModel.FindReadyByObjectKey(l.ctx, finalKey); err == nil {
		resp, err := l.instantComplete(owner, finalKey, input, existing)
		if err != nil {
			return nil, rpcerror.ToStatus(err)
		}
		return resp, nil
	} else if !errors.Is(err, model.ErrNotFound) {
		return nil, rpcerror.ToStatus(err)
	}

	// 未命中：落 pending 行（object_key 先记 tmp，digest_algo 留 0，confirm 时落最终 key），
	// 签发带 checksum 的 presigned PUT 到 tmp/{media_id}/{sha256}。
	mediaID, err := l.svcCtx.MediaIDGen.Next(0) // media HintBits=0（无单/群语义），hint 恒传 0。
	if err != nil {
		return nil, rpcerror.ToStatus(apperror.Internal("could not allocate media id"))
	}
	tmpKey := tmpObjectKey(mediaID, input.sha256)
	metadata, err := uploadMetadataJSON(input.sha256, in.GetWidth(), in.GetHeight())
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	purposeDB, _ := purposeToDB(input.purpose)
	created, err := l.svcCtx.MediaModel.CreateMediaObject(l.ctx, &model.MediaObjects{
		MediaId:          mediaID,
		UploaderId:       owner,
		Bucket:           bucket,
		ObjectKey:        tmpKey,
		OriginalFilename: input.filename,
		ContentType:      input.contentType,
		SizeBytes:        in.GetSizeBytes(),
		Purpose:          purposeDB,
		Status:           model.MediaStatusPending,
		Metadata:         metadata,
		DigestAlgo:       model.MediaDigestAlgoUnspecified,
	})
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}

	expiresAt := time.Now().UTC().Add(uploadURLTTL)
	uploadURL, err := l.svcCtx.Store.PresignPutWithChecksum(l.ctx, created.ObjectKey, input.contentType, in.GetSizeBytes(), input.sha256, uploadURLTTL)
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	return &media.CreateUploadIntentResponse{
		MediaId:   formatMediaID(created.MediaId),
		ObjectKey: finalKey,
		UploadUrl: uploadURL,
		ExpiresAt: expiresAt.UnixMilli(),
	}, nil
}

// instantComplete 处理秒传命中：用既有 ready 行的真实 size/content-type（按请求 purpose 复核限额，
// 防大文件经秒传绕过小限额）落一条新的 ready 行，返回 already_complete=true、无 upload_url。
func (l *CreateUploadIntentLogic) instantComplete(owner, finalKey string, input uploadIntentInput, existing *model.MediaObjects) (*media.CreateUploadIntentResponse, error) {
	if err := validatePurposeContent(input.purpose, existing.ContentType, existing.SizeBytes); err != nil {
		return nil, err
	}
	mediaID, err := l.svcCtx.MediaIDGen.Next(0)
	if err != nil {
		return nil, apperror.Internal("could not allocate media id")
	}
	metadata, err := uploadMetadataJSON(input.sha256, 0, 0)
	if err != nil {
		return nil, err
	}
	purposeDB, _ := purposeToDB(input.purpose)
	created, err := l.svcCtx.MediaModel.CreateMediaObject(l.ctx, &model.MediaObjects{
		MediaId:          mediaID,
		UploaderId:       owner,
		Bucket:           existing.Bucket,
		ObjectKey:        finalKey,
		OriginalFilename: input.filename,
		ContentType:      existing.ContentType,
		SizeBytes:        existing.SizeBytes,
		Purpose:          purposeDB,
		Status:           model.MediaStatusReady,
		Metadata:         metadata,
		DigestAlgo:       model.MediaDigestAlgoSHA256,
	})
	if err != nil {
		return nil, err
	}
	return &media.CreateUploadIntentResponse{
		MediaId:         formatMediaID(created.MediaId),
		ObjectKey:       finalKey,
		AlreadyComplete: true,
	}, nil
}
