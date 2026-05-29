package groups

import (
	"context"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/pkg/ctxuser"
	business "github.com/wujunhui99/agents_im/internal/logic"
	groupssvc "github.com/wujunhui99/agents_im/internal/servicecontext/groups"
	"github.com/wujunhui99/agents_im/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type ListGroupsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *groupssvc.ServiceContext
}

func NewListGroupsLogic(ctx context.Context, svcCtx *groupssvc.ServiceContext) *ListGroupsLogic {
	return &ListGroupsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListGroupsLogic) ListGroups(_ *types.ListGroupsReq) (*types.ListGroupsResp, error) {
	userID, err := ctxuser.UserID(l.ctx)
	if err != nil {
		return nil, err
	}

	result, err := l.svcCtx.GroupsLogic.ListGroups(l.ctx, business.ListGroupsRequest{UserID: userID})
	if err != nil {
		return nil, err
	}
	groups := make([]types.Group, 0, len(result.Groups))
	for _, group := range result.Groups {
		groups = append(groups, toGroup(group))
	}
	return &types.ListGroupsResp{
		Code:    string(apperror.CodeOK),
		Message: "ok",
		Data:    types.ListGroupsData{Groups: groups},
	}, nil
}
