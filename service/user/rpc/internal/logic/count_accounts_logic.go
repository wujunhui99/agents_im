package logic

import (
	"context"

	"github.com/wujunhui99/agents_im/pkg/rpcerror"
	"github.com/wujunhui99/agents_im/service/user/rpc/internal/svc"
	userpb "github.com/wujunhui99/agents_im/service/user/rpc/user"

	"github.com/zeromicro/go-zero/core/logx"
)

type CountAccountsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCountAccountsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CountAccountsLogic {
	return &CountAccountsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// CountAccounts 统计账号总数（管理后台 dashboard 用）。
func (l *CountAccountsLogic) CountAccounts(in *userpb.CountAccountsRequest) (*userpb.CountAccountsResponse, error) {
	count, err := l.svcCtx.Accounts.CountAccounts(l.ctx)
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	return &userpb.CountAccountsResponse{Count: count}, nil
}
