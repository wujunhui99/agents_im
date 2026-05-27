// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package user

import (
	"context"

	"github.com/wujunhui99/agents_im/service/user/api/internal/svc"
	"github.com/wujunhui99/agents_im/service/user/api/internal/types"
	userpb "github.com/wujunhui99/agents_im/service/user/rpc/user"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateAccountLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateAccountLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateAccountLogic {
	return &CreateAccountLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreateAccountLogic) CreateAccount(req *types.CreateUserReq) (resp *types.UserResp, err error) {
	userResp, err := l.svcCtx.UserRPC.CreateUser(l.ctx, &userpb.CreateUserRequest{
		Identifier:  req.Identifier,
		Email:       req.Email,
		DisplayName: req.DisplayName,
		Name:        req.Name,
		Gender:      req.Gender,
		BirthDate:   req.BirthDate,
		Region:      req.Region,
	})
	if err != nil {
		return nil, apiError(err)
	}
	return userRespFromRPC(userResp)
}
