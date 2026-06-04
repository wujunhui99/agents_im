package logic

import (
	"context"

	"github.com/wujunhui99/agents_im/common/share/rpcerror"
	"github.com/wujunhui99/agents_im/pkg/apperror"
	groups "github.com/wujunhui99/agents_im/service/groups/rpc/groups"
	"github.com/wujunhui99/agents_im/service/groups/rpc/internal/model"
	"github.com/wujunhui99/agents_im/service/groups/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type KickMemberLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewKickMemberLogic(ctx context.Context, svcCtx *svc.ServiceContext) *KickMemberLogic {
	return &KickMemberLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *KickMemberLogic) KickMember(in *groups.KickMemberRequest) (*groups.MemberResponse, error) {
	groupID, err := validateRequiredID(in.GetGroupId(), "group_id")
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	operatorUserID, err := validateRequiredID(in.GetOperatorUserId(), "operator_user_id")
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	userID, err := validateRequiredID(in.GetUserId(), "user_id")
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	if operatorUserID == userID {
		return nil, rpcerror.ToStatus(apperror.InvalidArgument("use leave group to remove yourself"))
	}

	if _, err := l.svcCtx.GroupsModel.FindOne(l.ctx, groupID); err != nil {
		return nil, rpcerror.ToStatus(notFoundAs(err, "group not found"))
	}
	operator, err := ensureCanManageGroup(l.ctx, l.svcCtx.GroupMembersModel, groupID, operatorUserID)
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	target, err := activeMember(l.ctx, l.svcCtx.GroupMembersModel, groupID, userID)
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	if target.Role == model.MemberRoleOwner {
		return nil, rpcerror.ToStatus(apperror.Forbidden("group owner cannot be kicked"))
	}
	if operator.Role == model.MemberRoleAdmin && target.Role != model.MemberRoleMember {
		return nil, rpcerror.ToStatus(apperror.Forbidden("group admin can only kick normal members"))
	}

	member, err := l.svcCtx.GroupMembersModel.SetMemberLeft(l.ctx, groupID, userID)
	if err != nil {
		return nil, rpcerror.ToStatus(notFoundAs(err, "member not found"))
	}
	return &groups.MemberResponse{Member: toGroupMember(member)}, nil
}
