package logic

import (
	"context"

	"github.com/wujunhui99/agents_im/common/share/rpcerror"
	"github.com/wujunhui99/agents_im/pkg/apperror"
	groups "github.com/wujunhui99/agents_im/service/groups/rpc/groups"
	"github.com/wujunhui99/agents_im/service/groups/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateGroupLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateGroupLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateGroupLogic {
	return &UpdateGroupLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *UpdateGroupLogic) UpdateGroup(in *groups.UpdateGroupRequest) (*groups.GroupResponse, error) {
	groupID, err := validateRequiredID(in.GetGroupId(), "group_id")
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	operatorUserID, err := validateRequiredID(in.GetOperatorUserId(), "operator_user_id")
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}

	group, err := l.svcCtx.GroupsModel.FindOne(l.ctx, groupID)
	if err != nil {
		return nil, rpcerror.ToStatus(notFoundAs(err, "group not found"))
	}
	operator, err := ensureCanManageGroup(l.ctx, l.svcCtx.GroupMembersModel, groupID, operatorUserID)
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}

	name := group.Name
	if in.GetName() != "" {
		name, err = validateGroupName(in.GetName())
		if err != nil {
			return nil, rpcerror.ToStatus(err)
		}
	}
	description := group.Description
	announcement := in.GetAnnouncement()
	if announcement == "" {
		announcement = in.GetDescription()
	}
	if announcement != "" {
		description, err = validateGroupDescription(announcement)
		if err != nil {
			return nil, rpcerror.ToStatus(err)
		}
	}
	if name == group.Name && description == group.Description {
		return nil, rpcerror.ToStatus(apperror.InvalidArgument("name or announcement is required"))
	}

	updated, err := l.svcCtx.GroupsModel.UpdateNameDescription(l.ctx, groupID, name, description)
	if err != nil {
		return nil, rpcerror.ToStatus(notFoundAs(err, "group not found"))
	}
	return &groups.GroupResponse{Group: toGroup(updated, memberRoleToString(operator.Role))}, nil
}
