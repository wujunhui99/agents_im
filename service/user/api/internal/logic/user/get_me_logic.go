// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package user

import (
	"context"

	"github.com/wujunhui99/agents_im/internal/ctxuser"
	"github.com/wujunhui99/agents_im/proto/userpb"
	"github.com/wujunhui99/agents_im/service/user/api/internal/svc"
	"github.com/wujunhui99/agents_im/service/user/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetMeLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetMeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetMeLogic {
	return &GetMeLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetMeLogic) GetMe(req *types.GetMeReq) (resp *types.UserResp, err error) {
	userID, err := ctxuser.UserID(l.ctx)
	if err != nil {
		return nil, err
	}
	userResp, err := l.svcCtx.UserRPC.GetUserByID(l.ctx, &userpb.GetUserByIDRequest{UserId: userID})
	if err != nil {
		return nil, apiError(err)
	}
	return userRespFromRPC(userResp)
}
