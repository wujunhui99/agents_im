package groups

import (
	"context"
	"github.com/wujunhui99/agents_im/internal/apperror"

	"github.com/wujunhui99/agents_im/internal/ctxuser"
	"github.com/wujunhui99/agents_im/service/groups/api/internal/svc"
	"github.com/wujunhui99/agents_im/service/groups/api/internal/types"
	groupspb "github.com/wujunhui99/agents_im/service/groups/rpc/groups"
	"github.com/zeromicro/go-zero/core/logx"
)

type ListGroupsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListGroupsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListGroupsLogic {
	return &ListGroupsLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *ListGroupsLogic) ListGroups(req *types.ListGroupsReq) (*types.ListGroupsResp, error) {
	userID, err := ctxuser.UserID(l.ctx)
	if err != nil {
		return nil, err
	}
	resp, err := l.svcCtx.GroupsRPC.ListGroups(l.ctx, &groupspb.ListGroupsRequest{UserId: userID})
	if err != nil {
		return nil, apiError(err)
	}
	items := make([]types.Group, 0, len(resp.GetGroups()))
	for _, item := range resp.GetGroups() {
		items = append(items, toGroup(item))
	}
	return &types.ListGroupsResp{Code: string(apperror.CodeOK), Message: "ok", Data: types.ListGroupsData{Groups: items}}, nil
}
