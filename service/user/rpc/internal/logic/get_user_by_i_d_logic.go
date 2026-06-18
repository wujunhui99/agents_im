package logic

import (
	"context"
	"strings"

	"github.com/wujunhui99/agents_im/pkg/rpcerror"
	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/service/user/rpc/internal/svc"
	userpb "github.com/wujunhui99/agents_im/service/user/rpc/user"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetUserByIDLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetUserByIDLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetUserByIDLogic {
	return &GetUserByIDLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetUserByIDLogic) GetUserByID(in *userpb.GetUserByIDRequest) (*userpb.UserResponse, error) {
	userID := strings.TrimSpace(in.GetUserId())
	if userID == "" {
		return nil, rpcerror.ToStatus(apperror.InvalidArgument("user_id is required"))
	}
	ap, err := l.svcCtx.Accounts.FindAccountProfileByID(l.ctx, userID)
	if err != nil {
		return nil, rpcerror.ToStatus(mapReadError(err))
	}
	return toUserResponse(ap), nil
}
