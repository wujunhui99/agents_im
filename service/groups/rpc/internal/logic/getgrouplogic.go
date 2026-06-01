package logic

import (
	"context"

	business "github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/common/share/rpcerror"
	groups "github.com/wujunhui99/agents_im/service/groups/rpc/groups"
	"github.com/wujunhui99/agents_im/service/groups/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetGroupLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetGroupLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetGroupLogic {
	return &GetGroupLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *GetGroupLogic) GetGroup(in *groups.GetGroupRequest) (*groups.GroupResponse, error) {
	result, err := l.svcCtx.GroupsLogic.GetGroup(l.ctx, business.GetGroupRequest{GroupID: in.GetGroupId(), RequesterUserID: in.GetRequesterUserId()})
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	return &groups.GroupResponse{Group: toGroup(result)}, nil
}
