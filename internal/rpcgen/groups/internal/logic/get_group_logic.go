package logic

import (
	"context"

	business "github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/rpcgen/groups/internal/svc"
	"github.com/wujunhui99/agents_im/internal/rpcgen/rpcerror"
	"github.com/wujunhui99/agents_im/proto/groupspb"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetGroupLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetGroupLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetGroupLogic {
	return &GetGroupLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetGroupLogic) GetGroup(in *groupspb.GetGroupRequest) (*groupspb.GroupResponse, error) {
	result, err := l.svcCtx.GroupsLogic.GetGroup(l.ctx, business.GetGroupRequest{
		GroupID: in.GetGroupId(),
	})
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	return &groupspb.GroupResponse{Group: toGroup(result)}, nil
}
