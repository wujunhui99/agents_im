package user

import (
	"context"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	business "github.com/wujunhui99/agents_im/internal/logic"
	usersvc "github.com/wujunhui99/agents_im/internal/servicecontext/user"
	"github.com/wujunhui99/agents_im/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type ExistsAccountLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *usersvc.ServiceContext
}

func NewExistsAccountLogic(ctx context.Context, svcCtx *usersvc.ServiceContext) *ExistsAccountLogic {
	return &ExistsAccountLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ExistsAccountLogic) ExistsAccount(req *types.ExistsReq) (*types.ExistsResp, error) {
	result, err := l.svcCtx.UserLogic.ExistsByIdentifier(l.ctx, business.ExistsByIdentifierRequest{
		Identifier: req.Identifier,
	})
	if err != nil {
		return nil, err
	}
	return &types.ExistsResp{
		Code:    string(apperror.CodeOK),
		Message: "ok",
		Data: types.ExistsData{
			Identifier: result.Identifier,
			Exists:     result.Exists,
		},
	}, nil
}
