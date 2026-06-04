package logic

import (
	"context"

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
	userID, err := validateRequiredID(in.GetUserId(), "user_id")
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	rows, err := l.svcCtx.GroupsModel.FindGroupsByMember(l.ctx, userID)
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	items := make([]*groups.Group, 0, len(rows))
	for _, g := range rows {
		items = append(items, toGroup(g, ""))
	}
	return &groups.ListGroupsResponse{Groups: items}, nil
}
