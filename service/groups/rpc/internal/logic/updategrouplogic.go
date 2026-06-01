package logic

import (
	"context"

	business "github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/common/share/rpcerror"
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
	result, err := l.svcCtx.GroupsLogic.UpdateGroup(l.ctx, business.UpdateGroupRequest{GroupID: in.GetGroupId(), OperatorUserID: in.GetOperatorUserId(), Name: in.GetName(), Description: in.GetDescription(), Announcement: in.GetAnnouncement()})
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	return &groups.GroupResponse{Group: toGroup(result)}, nil
}
