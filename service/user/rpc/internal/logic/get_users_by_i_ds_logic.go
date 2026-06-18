package logic

import (
	"context"
	"strings"

	"github.com/wujunhui99/agents_im/pkg/rpcerror"
	"github.com/wujunhui99/agents_im/service/user/rpc/internal/svc"
	userpb "github.com/wujunhui99/agents_im/service/user/rpc/user"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetUsersByIDsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetUsersByIDsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetUsersByIDsLogic {
	return &GetUsersByIDsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetUsersByIDs 批量获取用户资料：一条 WHERE id IN (...) 取代 N 次 GetUserByID。
// 不存在的 id 静默跳过，返回找到的子集（不保证顺序），调用方按需比对缺失。
func (l *GetUsersByIDsLogic) GetUsersByIDs(in *userpb.GetUsersByIDsRequest) (*userpb.GetUsersByIDsResponse, error) {
	ids := make([]string, 0, len(in.GetUserIds()))
	for _, id := range in.GetUserIds() {
		if trimmed := strings.TrimSpace(id); trimmed != "" {
			ids = append(ids, trimmed)
		}
	}

	rows, err := l.svcCtx.Accounts.ListAccountProfilesByIDs(l.ctx, ids)
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}

	users := make([]*userpb.UserEntity, 0, len(rows))
	for _, ap := range rows {
		users = append(users, toUserEntity(ap))
	}
	return &userpb.GetUsersByIDsResponse{Users: users}, nil
}
