// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package admin

import (
	"context"

	"github.com/wujunhui99/agents_im/common/share/rpcerror"
	"github.com/wujunhui99/agents_im/service/admin/api/internal/svc"
	"github.com/wujunhui99/agents_im/service/admin/api/internal/types"
	adminpb "github.com/wujunhui99/agents_im/service/admin/rpc/admin"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetUserConversationsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetUserConversationsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetUserConversationsLogic {
	return &GetUserConversationsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetUserConversationsLogic) GetUserConversations(req *types.AdminUserReq) (resp *types.AdminUserConversationsResp, err error) {
	out, err := l.svcCtx.AdminRPC.GetUserConversations(l.ctx, &adminpb.UserConversationsRequest{AccountId: req.AccountID})
	if err != nil {
		return nil, rpcerror.FromStatus(err)
	}
	return &types.AdminUserConversationsResp{Code: codeOK, Message: messageOK, Data: types.AdminUserConversationsData{Conversations: adminConversations(out.GetConversations())}}, nil
}
