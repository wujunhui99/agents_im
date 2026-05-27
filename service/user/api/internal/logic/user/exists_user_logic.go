// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package user

import (
	"context"

	"github.com/wujunhui99/agents_im/internal/apperror"
	"github.com/wujunhui99/agents_im/proto/userpb"
	"github.com/wujunhui99/agents_im/service/user/api/internal/svc"
	"github.com/wujunhui99/agents_im/service/user/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ExistsUserLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewExistsUserLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ExistsUserLogic {
	return &ExistsUserLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ExistsUserLogic) ExistsUser(req *types.ExistsReq) (resp *types.ExistsResp, err error) {
	result, err := l.svcCtx.UserRPC.ExistsByIdentifier(l.ctx, &userpb.ExistsByIdentifierRequest{
		Identifier: req.Identifier,
	})
	if err != nil {
		return nil, apiError(err)
	}
	return &types.ExistsResp{
		Code:    string(apperror.CodeOK),
		Message: "ok",
		Data: types.ExistsData{
			Identifier: result.GetIdentifier(),
			Exists:     result.GetExists(),
		},
	}, nil
}
