// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package admin

import (
	"context"

	"github.com/wujunhui99/agents_im/pkg/rpcerror"
	"github.com/wujunhui99/agents_im/service/admin/api/internal/svc"
	"github.com/wujunhui99/agents_im/service/admin/api/internal/types"
	adminpb "github.com/wujunhui99/agents_im/service/admin/rpc/admin"

	"github.com/zeromicro/go-zero/core/logx"
)

type SearchUsersLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSearchUsersLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SearchUsersLogic {
	return &SearchUsersLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SearchUsersLogic) SearchUsers(req *types.AdminUserSearchReq) (resp *types.AdminUserSearchResp, err error) {
	out, err := l.svcCtx.AdminRPC.SearchUsers(l.ctx, &adminpb.UserSearchRequest{
		Query: req.Query,
		Limit: int32(req.Limit),
	})
	if err != nil {
		return nil, rpcerror.FromStatus(err)
	}
	return &types.AdminUserSearchResp{Code: codeOK, Message: messageOK, Data: types.AdminUserSearchData{Users: adminUsers(out.GetUsers())}}, nil
}
