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

// CompleteUpload 内容寻址上传第二步：取回 OSS 已校验的 ChecksumSHA256，比对 == 客户端上报 sha256，
// 一致则 copy tmp/{upload_id}/{sha256} → agents_im/{sha256} + 删 tmp，落 ready（EPIC #527 §3）。
// 幂等可重试：行已 ready 直接返回；tmp 已被前次部分执行删除则用 finalKey 续完成。
func (l *CompleteUploadLogic) CompleteUpload(in *media.CompleteUploadRequest) (*media.CompleteUploadResponse, error) {
	mediaID, err := parseMediaID(in.GetMediaId())
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	obj, err := mediaForOwner(l.ctx, l.svcCtx, in.GetOwnerUserId(), mediaID)
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	// 幂等：已 ready（秒传命中 / confirm 重试）直接回放，不再碰 OSS。
	if obj.Status == model.MediaStatusReady {
		return &media.CompleteUploadResponse{Media: toPBMediaObject(obj)}, nil
	}
	if obj.Status != model.MediaStatusPending {
		return nil, rpcerror.ToStatus(apperror.InvalidArgument("media object is not pending"))
	}
	if l.svcCtx.Store == nil {
		return nil, rpcerror.ToStatus(apperror.Internal("object store is not configured"))
	}

	sha256, ok := sha256FromTmpKey(obj.ObjectKey)
	if !ok {
		return nil, rpcerror.ToStatus(apperror.Internal("pending media object_key is not a content-addressed tmp key"))
	}
	finalKey := finalObjectKey(sha256)

	if err := l.verifyAndRename(obj, sha256, finalKey); err != nil {
		return nil, rpcerror.ToStatus(err)
	}

	updated, err := l.svcCtx.MediaModel.MarkReady(l.ctx, obj.MediaId, finalKey, model.MediaDigestAlgoSHA256)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, rpcerror.ToStatus(apperror.NotFound("media object not found"))
		}
		return nil, rpcerror.ToStatus(err)
	}
	return &media.CompleteUploadResponse{Media: toPBMediaObject(updated)}, nil
}

// verifyAndRename 比对 OSS checksum 后把 tmp 对象改名到 finalKey。校验是 OSS 的职责：media 只读
// HeadObject 取回的 ChecksumSHA256，不回算字节（EPIC #527 §3）。copy/delete 幂等可重试。
func (l *CompleteUploadLogic) verifyAndRename(obj *model.MediaObjects, sha256, finalKey string) error {
	info, err := l.svcCtx.Store.StatObject(l.ctx, obj.ObjectKey)
	if err != nil {
		if errors.Is(err, objectstorage.ErrObjectNotFound) {
			// tmp 不在了：可能前次 confirm 已 copy+delete 但 MarkReady 失败。若 finalKey 已就位，
			// 视为 rename 已完成、续落 ready；否则确属对象缺失。
			if finfo, ferr := l.svcCtx.Store.StatObject(l.ctx, finalKey); ferr == nil {
				return l.checkObject(finfo, sha256, obj.SizeBytes)
			}
			return apperror.NotFound("uploaded object not found")
		}
		return err
	}
	if err := l.checkObject(info, sha256, obj.SizeBytes); err != nil {
		return err
	}
	// copy tmp → final（OSS 内部 copy，无外部流量），再删 tmp。两步均幂等：重复 copy 内容相同，
	// 删已删对象不报错。
	if err := l.svcCtx.Store.CopyObject(l.ctx, obj.ObjectKey, finalKey); err != nil {
		return err
	}
	if err := l.svcCtx.Store.RemoveObject(l.ctx, obj.ObjectKey); err != nil {
		// 改名主体（copy）已成功，删 tmp 失败只留垃圾、不影响正确性，交后台 tmp 清理兜底。
		logx.WithContext(l.ctx).Errorf("complete upload: remove tmp object %q failed (left for sweeper): %v", obj.ObjectKey, err)
	}
	return nil
}

// checkObject 校验 OSS 对象的 size 与 sha256 checksum；checksum 缺失/不符即拒（media 不回算兜底）。
func (l *CompleteUploadLogic) checkObject(info objectstorage.ObjectInfo, sha256 string, wantSize int64) error {
	if info.SizeBytes != wantSize {
		return apperror.InvalidArgument("uploaded object size does not match upload intent")
	}
	if info.ChecksumSHA256 == "" {
		// OSS 未返回 server-side checksum：要么对象未带 x-amz-checksum-sha256 入库，要么 OSS 版本
		// 不支持。media 不回算兜底（职责划分），显式失败、报告根因（EPIC #527 §3 gating）。
		return apperror.Internal("object storage returned no sha256 checksum; cannot confirm integrity")
	}
	if !sha256ChecksumMatches(info.ChecksumSHA256, sha256) {
		return apperror.InvalidArgument("uploaded object sha256 does not match upload intent")
	}
	return nil
}
