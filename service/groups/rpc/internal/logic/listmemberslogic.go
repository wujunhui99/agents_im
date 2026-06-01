package logic

import (
	"context"

	business "github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/common/share/rpcerror"
	groups "github.com/wujunhui99/agents_im/service/groups/rpc/groups"
	"github.com/wujunhui99/agents_im/service/groups/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListMembersLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListMembersLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListMembersLogic {
	return &ListMembersLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *ListMembersLogic) ListMembers(in *groups.ListMembersRequest) (*groups.ListMembersResponse, error) {
	result, err := l.svcCtx.GroupsLogic.ListMembers(l.ctx, business.ListMembersRequest{GroupID: in.GetGroupId(), RequesterUserID: in.GetRequesterUserId()})
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	items := make([]*groups.GroupMember, 0, len(result.Members))
	for _, item := range result.Members {
		items = append(items, toGroupMember(item))
	}
	return &groups.ListMembersResponse{GroupId: result.GroupID, Members: items}, nil
}
