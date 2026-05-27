package logic

import (
	"context"

	business "github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/rpcgen/rpcerror"
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
	result, err := l.svcCtx.GroupsLogic.LeaveGroup(l.ctx, business.LeaveGroupRequest{GroupID: in.GetGroupId(), UserID: in.GetUserId()})
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	return toMemberResponse(result), nil
}
