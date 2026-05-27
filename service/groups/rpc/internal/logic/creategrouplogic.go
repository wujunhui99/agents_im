package logic

import (
	"context"

	business "github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/rpcgen/rpcerror"
	groups "github.com/wujunhui99/agents_im/service/groups/rpc/groups"
	"github.com/wujunhui99/agents_im/service/groups/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateGroupLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateGroupLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateGroupLogic {
	return &CreateGroupLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *CreateGroupLogic) CreateGroup(in *groups.CreateGroupRequest) (*groups.GroupResponse, error) {
	result, err := l.svcCtx.GroupsLogic.CreateGroup(l.ctx, business.CreateGroupRequest{CreatorUserID: in.GetCreatorUserId(), Name: in.GetName(), Description: in.GetDescription(), MemberUserIDs: in.GetMemberUserIds()})
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	return &groups.GroupResponse{Group: toGroup(result)}, nil
}
