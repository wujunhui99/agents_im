package logic

import (
	"context"

	"github.com/wujunhui99/agents_im/pkg/rpcerror"
	"github.com/wujunhui99/agents_im/service/user/rpc/internal/svc"
	userpb "github.com/wujunhui99/agents_im/service/user/rpc/user"

	"github.com/zeromicro/go-zero/core/logx"
)

const (
	defaultSearchAccountsLimit = 20
	maxSearchAccountsLimit     = 100
)

type SearchAccountsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSearchAccountsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SearchAccountsLogic {
	return &SearchAccountsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// SearchAccounts 按 query 模糊搜账号（管理后台跨域只读，经属主 user-rpc）。
func (l *SearchAccountsLogic) SearchAccounts(in *userpb.SearchAccountsRequest) (*userpb.SearchAccountsResponse, error) {
	rows, err := l.svcCtx.Accounts.SearchAccountProfiles(l.ctx, in.GetQuery(), normalizeSearchLimit(int(in.GetLimit())))
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}

	users := make([]*userpb.UserEntity, 0, len(rows))
	for _, ap := range rows {
		users = append(users, toUserEntity(ap))
	}
	return &userpb.SearchAccountsResponse{Users: users}, nil
}

// normalizeSearchLimit 复刻 internal/repository.normalizeAdminLimit（默认 20，上限 100）。
func normalizeSearchLimit(limit int) int {
	if limit <= 0 {
		return defaultSearchAccountsLimit
	}
	if limit > maxSearchAccountsLimit {
		return maxSearchAccountsLimit
	}
	return limit
}
