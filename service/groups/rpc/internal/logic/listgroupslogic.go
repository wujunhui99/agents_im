package logic

import (
	"context"

	business "github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/common/share/rpcerror"
	groups "github.com/wujunhui99/agents_im/service/groups/rpc/groups"
	"github.com/wujunhui99/agents_im/service/groups/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListGroupsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListGroupsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListGroupsLogic {
	return &ListGroupsLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *ListGroupsLogic) ListGroups(in *groups.ListGroupsRequest) (*groups.ListGroupsResponse, error) {
	result, err := l.svcCtx.GroupsLogic.ListGroups(l.ctx, business.ListGroupsRequest{UserID: in.GetUserId()})
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	items := make([]*groups.Group, 0, len(result.Groups))
	for _, item := range result.Groups {
		items = append(items, toGroup(item))
	}
	return &groups.ListGroupsResponse{Groups: items}, nil
}
