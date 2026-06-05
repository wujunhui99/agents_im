package logic

import (
	"context"
	"time"

	"github.com/wujunhui99/agents_im/common/share/rpcerror"
	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/service/media/rpc/internal/svc"
	"github.com/wujunhui99/agents_im/service/media/rpc/media"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetAvatarDisplayURLLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetAvatarDisplayURLLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetAvatarDisplayURLLogic {
	return &GetAvatarDisplayURLLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetAvatarDisplayURLLogic) GetAvatarDisplayURL(in *media.GetAvatarDisplayURLRequest) (*media.GetDownloadURLResponse, error) {
	mediaID, err := validateMediaIDComponent(in.GetMediaId(), "media_id")
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	obj, err := mediaByID(l.ctx, l.svcCtx, mediaID)
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	if err := validateAvatarMediaObject(obj); err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	if l.svcCtx.Store == nil {
		return nil, rpcerror.ToStatus(apperror.Internal("object store is not configured"))
	}
	expiresAt := time.Now().UTC().Add(avatarDownloadURLTTL)
	downloadURL, err := l.svcCtx.Store.PresignGet(l.ctx, obj.ObjectKey, avatarDownloadURLTTL)
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	return &media.GetDownloadURLResponse{
		MediaId:     obj.MediaId,
		DownloadUrl: downloadURL,
		ExpiresAt:   expiresAt.UnixMilli(),
	}, nil
}
