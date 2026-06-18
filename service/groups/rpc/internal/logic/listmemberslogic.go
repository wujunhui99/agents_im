package logic

import (
	"context"

	"github.com/wujunhui99/agents_im/pkg/rpcerror"
	"github.com/wujunhui99/agents_im/pkg/apperror"
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
	groupID, err := validateRequiredID(in.GetGroupId(), "group_id")
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	requesterUserID, err := validateOptionalID(in.GetRequesterUserId(), "requester_user_id")
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}

	if _, err := l.svcCtx.GroupsModel.FindOne(l.ctx, groupID); err != nil {
		return nil, rpcerror.ToStatus(notFoundAs(err, "group not found"))
	}
	members, err := l.svcCtx.GroupMembersModel.FindActiveByGroup(l.ctx, groupID)
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	if requesterUserID != "" && !containsActiveMember(members, requesterUserID) {
		return nil, rpcerror.ToStatus(apperror.Forbidden("requester is not a group member"))
	}

	items := make([]*groups.GroupMember, 0, len(members))
	for _, m := range members {
		items = append(items, toGroupMember(m))
	}
	return &groups.ListMembersResponse{GroupId: groupID, Members: items}, nil
}
