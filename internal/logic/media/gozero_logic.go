package media

import (
	"context"

	"github.com/wujunhui99/agents_im/internal/apperror"
	"github.com/wujunhui99/agents_im/internal/ctxuser"
	business "github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/svc"
	"github.com/wujunhui99/agents_im/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type CreateUploadIntentLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateUploadIntentLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateUploadIntentLogic {
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

type CompleteUploadLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCompleteUploadLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CompleteUploadLogic {
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

type GetDownloadURLLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetDownloadURLLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetDownloadURLLogic {
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

type GetAvatarLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetAvatarLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetAvatarLogic {
	return &GetAvatarLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetAvatarLogic) GetAvatar(req *types.GetMediaDownloadURLReq) (business.GetMediaDownloadURLResponse, error) {
	if l.svcCtx.MediaLogic == nil {
		return business.GetMediaDownloadURLResponse{}, apperror.Internal("media logic is not configured")
	}
	return l.svcCtx.MediaLogic.GetAvatarDisplayURL(l.ctx, req.MediaID)
}

func toMediaObject(media business.MediaObject) types.MediaObject {
	return types.MediaObject{
		MediaID:          media.MediaID,
		OwnerUserID:      media.OwnerUserID,
		Bucket:           media.Bucket,
		ObjectKey:        media.ObjectKey,
		SHA256:           media.SHA256,
		ContentType:      media.ContentType,
		SizeBytes:        media.SizeBytes,
		Width:            media.Width,
		Height:           media.Height,
		OriginalFilename: media.OriginalFilename,
		Purpose:          media.Purpose,
		Status:           media.Status,
		CreatedAt:        media.CreatedAt,
		UpdatedAt:        media.UpdatedAt,
	}
}
