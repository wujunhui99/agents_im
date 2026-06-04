package logic

import (
	"context"

	"github.com/wujunhui99/agents_im/common/share/rpcerror"
	groups "github.com/wujunhui99/agents_im/service/groups/rpc/groups"
	"github.com/wujunhui99/agents_im/service/groups/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type JoinGroupLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewJoinGroupLogic(ctx context.Context, svcCtx *svc.ServiceContext) *JoinGroupLogic {
	return &JoinGroupLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *JoinGroupLogic) JoinGroup(in *groups.JoinGroupRequest) (*groups.MemberResponse, error) {
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

	resp, err := addActiveMember(l.ctx, l.svcCtx.GroupMembersModel, group, userID)
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	return resp, nil
}
