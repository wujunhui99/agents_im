package logic

import (
	"context"

	business "github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/rpcgen/groups/internal/svc"
	"github.com/wujunhui99/agents_im/internal/rpcgen/rpcerror"
	"github.com/wujunhui99/agents_im/proto/groupspb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListMembersLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListMembersLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListMembersLogic {
	return &ListMembersLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListMembersLogic) ListMembers(in *groupspb.ListMembersRequest) (*groupspb.ListMembersResponse, error) {
	result, err := l.svcCtx.GroupsLogic.ListMembers(l.ctx, business.ListMembersRequest{
		GroupID: in.GetGroupId(),
	})
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}

	members := make([]*groupspb.GroupMember, 0, len(result.Members))
	for _, member := range result.Members {
		members = append(members, toGroupMember(member))
	}
	return &groupspb.ListMembersResponse{
		GroupId: result.GroupID,
		Members: members,
	}, nil
}
