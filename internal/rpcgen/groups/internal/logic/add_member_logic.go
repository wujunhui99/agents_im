package logic

import (
	"context"

	business "github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/rpcgen/groups/internal/svc"
	"github.com/wujunhui99/agents_im/internal/rpcgen/rpcerror"
	"github.com/wujunhui99/agents_im/proto/groupspb"

	"github.com/zeromicro/go-zero/core/logx"
)

type AddMemberLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAddMemberLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AddMemberLogic {
	return &AddMemberLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *AddMemberLogic) AddMember(in *groupspb.AddMemberRequest) (*groupspb.MemberResponse, error) {
	result, err := l.svcCtx.GroupsLogic.AddMember(l.ctx, business.AddMemberRequest{
		GroupID:        in.GetGroupId(),
		OperatorUserID: in.GetOperatorUserId(),
		UserID:         in.GetUserId(),
	})
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	return &groupspb.MemberResponse{
		Member:        toGroupMember(result.Member),
		AlreadyMember: result.AlreadyMember,
	}, nil
}
