package logic

import (
	"context"

	business "github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/rpcgen/groups/internal/svc"
	"github.com/wujunhui99/agents_im/internal/rpcgen/rpcerror"
	"github.com/wujunhui99/agents_im/proto/groupspb"

	"github.com/zeromicro/go-zero/core/logx"
)

type JoinGroupLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewJoinGroupLogic(ctx context.Context, svcCtx *svc.ServiceContext) *JoinGroupLogic {
	return &JoinGroupLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *JoinGroupLogic) JoinGroup(in *groupspb.JoinGroupRequest) (*groupspb.MemberResponse, error) {
	result, err := l.svcCtx.GroupsLogic.JoinGroup(l.ctx, business.JoinGroupRequest{
		GroupID: in.GetGroupId(),
		UserID:  in.GetUserId(),
	})
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	return &groupspb.MemberResponse{
		Member:        toGroupMember(result.Member),
		AlreadyMember: result.AlreadyMember,
	}, nil
}
