package logic

import (
	"context"

	business "github.com/wujunhui99/agents_im/internal/logic"
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
	profile, err := l.svcCtx.UserLogic.GetUserByIdentifier(l.ctx, business.GetUserByIdentifierRequest{
		Identifier: in.GetIdentifier(),
	})
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	return toUserResponse(profile), nil
}
