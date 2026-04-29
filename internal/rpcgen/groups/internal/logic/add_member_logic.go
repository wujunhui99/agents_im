package logic

import (
	"context"

	"github.com/wujunhui99/agents_im/internal/rpcgen/groups/internal/svc"
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
	// todo: add your logic here and delete this line

	return &groupspb.MemberResponse{}, nil
}
