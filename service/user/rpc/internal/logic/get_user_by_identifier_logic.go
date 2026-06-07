package logic

import (
	"context"

	"github.com/wujunhui99/agents_im/common/share/rpcerror"
	"github.com/wujunhui99/agents_im/service/user/rpc/internal/svc"
	userpb "github.com/wujunhui99/agents_im/service/user/rpc/user"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetUserByIdentifierLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetUserByIdentifierLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetUserByIdentifierLogic {
	return &GetUserByIdentifierLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetUserByIdentifierLogic) GetUserByIdentifier(in *userpb.GetUserByIdentifierRequest) (*userpb.UserResponse, error) {
	identifier, err := validateIdentifier(in.GetIdentifier())
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	ap, err := l.svcCtx.Accounts.FindAccountProfileByIdentifier(l.ctx, identifier)
	if err != nil {
		return nil, rpcerror.ToStatus(mapReadError(err))
	}
	return toUserResponse(ap), nil
}
