package logic

import (
	"context"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/pkg/rpcerror"
	groups "github.com/wujunhui99/agents_im/service/groups/rpc/groups"
	"github.com/wujunhui99/agents_im/service/groups/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type LeaveGroupLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewLeaveGroupLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LeaveGroupLogic {
	return &LeaveGroupLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *LeaveGroupLogic) LeaveGroup(in *groups.LeaveGroupRequest) (*groups.MemberResponse, error) {
	groupID, err := validateRequiredID(in.GetGroupId(), "group_id")
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	userID, err := validateRequiredID(in.GetUserId(), "user_id")
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}

	group, err := l.svcCtx.GroupsModel.FindOne(l.ctx, groupID)
	if err != nil {
		return nil, rpcerror.ToStatus(notFoundAs(err, "group not found"))
	}
	// owner 是唯一 active 成员时不能退群。
	if group.CreatorAccountId == userID {
		members, err := l.svcCtx.GroupMembersModel.FindActiveByGroup(l.ctx, groupID)
		if err != nil {
			return nil, rpcerror.ToStatus(err)
		}
		if containsActiveMember(members, userID) && len(members) <= 1 {
			return nil, rpcerror.ToStatus(apperror.Forbidden("group owner cannot leave as the only active member"))
		}
	}

	member, err := l.svcCtx.GroupMembersModel.SetMemberLeft(l.ctx, groupID, userID)
	if err != nil {
		return nil, rpcerror.ToStatus(notFoundAs(err, "member not found"))
	}
	return &groups.MemberResponse{Member: toGroupMember(member)}, nil
}
