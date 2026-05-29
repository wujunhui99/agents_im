package groups

import (
	"context"
	"github.com/wujunhui99/agents_im/pkg/apperror"

	"github.com/wujunhui99/agents_im/pkg/ctxuser"
	"github.com/wujunhui99/agents_im/service/groups/api/internal/svc"
	"github.com/wujunhui99/agents_im/service/groups/api/internal/types"
	groupspb "github.com/wujunhui99/agents_im/service/groups/rpc/groups"
	"github.com/zeromicro/go-zero/core/logx"
)

type ListMembersLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListMembersLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListMembersLogic {
	return &ListMembersLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *ListMembersLogic) ListMembers(req *types.ListMembersReq) (*types.ListMembersResp, error) {
	userID, err := ctxuser.UserID(l.ctx)
	if err != nil {
		return nil, err
	}
	resp, err := l.svcCtx.GroupsRPC.ListMembers(l.ctx, &groupspb.ListMembersRequest{GroupId: req.GroupID, RequesterUserId: userID})
	if err != nil {
		return nil, apiError(err)
	}
	items := make([]types.GroupMember, 0, len(resp.GetMembers()))
	for _, item := range resp.GetMembers() {
		items = append(items, toGroupMember(item))
	}
	return &types.ListMembersResp{Code: string(apperror.CodeOK), Message: "ok", Data: types.ListMembersData{GroupID: resp.GetGroupId(), Members: items}}, nil
}
