package groups

import (
	"context"

	"github.com/wujunhui99/agents_im/internal/ctxuser"
	business "github.com/wujunhui99/agents_im/internal/logic"
	groupssvc "github.com/wujunhui99/agents_im/internal/servicecontext/groups"
	"github.com/wujunhui99/agents_im/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateGroupLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *groupssvc.ServiceContext
}

func NewUpdateGroupLogic(ctx context.Context, svcCtx *groupssvc.ServiceContext) *UpdateGroupLogic {
	return &UpdateGroupLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateGroupLogic) UpdateGroup(req *types.UpdateGroupReq) (*types.GroupResp, error) {
	userID, err := ctxuser.UserID(l.ctx)
	if err != nil {
		return nil, err
	}

	group, err := l.svcCtx.GroupsLogic.UpdateGroup(l.ctx, business.UpdateGroupRequest{
		GroupID:        req.GroupID,
		OperatorUserID: userID,
		Name:           req.Name,
		Description:    req.Description,
		Announcement:   req.Announcement,
	})
	if err != nil {
		return nil, err
	}
	return groupResp(group), nil
}
