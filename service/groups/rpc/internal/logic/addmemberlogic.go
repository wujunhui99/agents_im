package logic

import (
	"context"

	business "github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/rpcgen/rpcerror"
	groups "github.com/wujunhui99/agents_im/service/groups/rpc/groups"
	"github.com/wujunhui99/agents_im/service/groups/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type AddMemberLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAddMemberLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AddMemberLogic {
	return &AddMemberLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *AddMemberLogic) AddMember(in *groups.AddMemberRequest) (*groups.MemberResponse, error) {
	result, err := l.svcCtx.GroupsLogic.AddMember(l.ctx, business.AddMemberRequest{GroupID: in.GetGroupId(), OperatorUserID: in.GetOperatorUserId(), UserID: in.GetUserId()})
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	return toMemberResponse(result), nil
}
