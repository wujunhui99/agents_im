package user

import (
	"context"

	business "github.com/wujunhui99/agents_im/internal/logic"
	usersvc "github.com/wujunhui99/agents_im/internal/servicecontext/user"
	"github.com/wujunhui99/agents_im/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type CreateAccountLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *usersvc.ServiceContext
}

func NewCreateAccountLogic(ctx context.Context, svcCtx *usersvc.ServiceContext) *CreateAccountLogic {
	return &CreateAccountLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreateAccountLogic) CreateAccount(req *types.CreateUserReq) (*types.UserResp, error) {
	profile, err := l.svcCtx.UserLogic.CreateUser(l.ctx, business.CreateUserRequest{
		Identifier:  req.Identifier,
		DisplayName: req.DisplayName,
		Name:        req.Name,
		Gender:      req.Gender,
		BirthDate:   req.BirthDate,
		Region:      req.Region,
	})
	if err != nil {
		return nil, err
	}
	return userResp(profile), nil
}
