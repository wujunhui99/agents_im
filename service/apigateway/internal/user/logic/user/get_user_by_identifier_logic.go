// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package user

import (
	"context"

	"github.com/wujunhui99/agents_im/service/apigateway/internal/user/svc"
	"github.com/wujunhui99/agents_im/service/apigateway/internal/user/types"
	userpb "github.com/wujunhui99/agents_im_protocol/user/v1"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetUserByIdentifierLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetUserByIdentifierLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetUserByIdentifierLogic {
	return &GetUserByIdentifierLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetUserByIdentifierLogic) GetUserByIdentifier(req *types.GetUserByIdentifierReq) (resp *types.UserResp, err error) {
	userResp, err := l.svcCtx.UserRPC.GetUserByIdentifier(l.ctx, &userpb.GetUserByIdentifierRequest{
		Identifier: req.Identifier,
	})
	if err != nil {
		return nil, apiError(err)
	}
	return userRespFromRPC(userResp)
}
