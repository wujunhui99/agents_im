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

type GetDownloadURLLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetDownloadURLLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetDownloadURLLogic {
	return &GetDownloadURLLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetDownloadURLLogic) GetDownloadURL(in *media.GetDownloadURLRequest) (*media.GetDownloadURLResponse, error) {
	requester := in.GetRequesterUserId()
	if requester == "" {
		requester = in.GetOwnerUserId()
	}
	requester, err := validateMediaIDComponent(requester, "owner_user_id")
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	mediaID, err := parseMediaID(in.GetMediaId())
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	obj, err := mediaByID(l.ctx, l.svcCtx, mediaID)
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	allowed, err := requesterCanAccessMedia(l.ctx, l.svcCtx, requester, obj)
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	if !allowed {
		return nil, rpcerror.ToStatus(apperror.Forbidden("media object is not accessible by requester"))
	}
	if obj.Status != model.MediaStatusReady {
		return nil, rpcerror.ToStatus(apperror.InvalidArgument("media object is not ready"))
	}
	if l.svcCtx.Store == nil {
		return nil, rpcerror.ToStatus(apperror.Internal("object store is not configured"))
	}
	expiresAt := time.Now().UTC().Add(downloadURLTTL)
	downloadURL, err := l.svcCtx.Store.PresignGet(l.ctx, obj.ObjectKey, downloadURLTTL)
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	return &media.GetDownloadURLResponse{
		MediaId:     formatMediaID(obj.MediaId),
		DownloadUrl: downloadURL,
		ExpiresAt:   expiresAt.UnixMilli(),
	}, nil
}
