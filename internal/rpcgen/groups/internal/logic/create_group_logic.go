package logic

import (
	"context"

	business "github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/rpcgen/groups/internal/svc"
	"github.com/wujunhui99/agents_im/internal/rpcgen/rpcerror"
	"github.com/wujunhui99/agents_im/proto/groupspb"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateGroupLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateGroupLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateGroupLogic {
	return &CreateGroupLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CreateGroupLogic) CreateGroup(in *groupspb.CreateGroupRequest) (*groupspb.GroupResponse, error) {
	result, err := l.svcCtx.GroupsLogic.CreateGroup(l.ctx, business.CreateGroupRequest{
		CreatorUserID: in.GetCreatorUserId(),
		Name:          in.GetName(),
		Description:   in.GetDescription(),
	})
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	return &groupspb.GroupResponse{Group: toGroup(result)}, nil
}
