package logic

import (
	"context"

	"github.com/wujunhui99/agents_im/pkg/rpcerror"
	groups "github.com/wujunhui99/agents_im/service/groups/rpc/groups"
	"github.com/wujunhui99/agents_im/service/groups/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetGroupLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetGroupLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetGroupLogic {
	return &GetGroupLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *GetGroupLogic) GetGroup(in *groups.GetGroupRequest) (*groups.GroupResponse, error) {
	groupID, err := validateRequiredID(in.GetGroupId(), "group_id")
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	requesterUserID, err := validateOptionalID(in.GetRequesterUserId(), "requester_user_id")
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}

	group, err := l.svcCtx.GroupsModel.FindOne(l.ctx, groupID)
	if err != nil {
		return nil, rpcerror.ToStatus(notFoundAs(err, "group not found"))
	}

	role := ""
	if requesterUserID != "" {
		member, err := activeMember(l.ctx, l.svcCtx.GroupMembersModel, groupID, requesterUserID)
		if err != nil {
			return nil, rpcerror.ToStatus(err)
		}
		role = memberRoleToString(member.Role)
	}
	return &groups.GroupResponse{Group: toGroup(group, role)}, nil
}
