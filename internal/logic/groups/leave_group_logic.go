package groups

import (
	"context"

	"github.com/wujunhui99/agents_im/internal/ctxuser"
	business "github.com/wujunhui99/agents_im/internal/logic"
	groupssvc "github.com/wujunhui99/agents_im/internal/servicecontext/groups"
	"github.com/wujunhui99/agents_im/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type LeaveGroupLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *groupssvc.ServiceContext
}

func NewLeaveGroupLogic(ctx context.Context, svcCtx *groupssvc.ServiceContext) *LeaveGroupLogic {
	return &LeaveGroupLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *LeaveGroupLogic) LeaveGroup(req *types.LeaveGroupReq) (*types.MemberResp, error) {
	userID, err := ctxuser.UserID(l.ctx)
	if err != nil {
		return nil, err
	}

	result, err := l.svcCtx.GroupsLogic.LeaveGroup(l.ctx, business.LeaveGroupRequest{
		GroupID: req.GroupID,
		UserID:  userID,
	})
	if err != nil {
		return nil, err
	}
	return memberResp(result), nil
}
