package groups

import (
	"context"

	"github.com/wujunhui99/agents_im/internal/ctxuser"
	business "github.com/wujunhui99/agents_im/internal/logic"
	groupssvc "github.com/wujunhui99/agents_im/internal/servicecontext/groups"
	"github.com/wujunhui99/agents_im/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetGroupLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *groupssvc.ServiceContext
}

func NewGetGroupLogic(ctx context.Context, svcCtx *groupssvc.ServiceContext) *GetGroupLogic {
	return &GetGroupLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetGroupLogic) GetGroup(req *types.GetGroupReq) (*types.GroupResp, error) {
	userID, err := ctxuser.UserID(l.ctx)
	if err != nil {
		return nil, err
	}

	group, err := l.svcCtx.GroupsLogic.GetGroup(l.ctx, business.GetGroupRequest{
		GroupID:         req.GroupID,
		RequesterUserID: userID,
	})
	if err != nil {
		return nil, err
	}
	return groupResp(group), nil
}
