package groups

import (
	"context"

	"github.com/wujunhui99/agents_im/internal/ctxuser"
	business "github.com/wujunhui99/agents_im/internal/logic"
	groupssvc "github.com/wujunhui99/agents_im/internal/servicecontext/groups"
	"github.com/wujunhui99/agents_im/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type CreateGroupLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *groupssvc.ServiceContext
}

func NewCreateGroupLogic(ctx context.Context, svcCtx *groupssvc.ServiceContext) *CreateGroupLogic {
	return &CreateGroupLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreateGroupLogic) CreateGroup(req *types.CreateGroupReq) (*types.GroupResp, error) {
	userID, err := ctxuser.UserID(l.ctx)
	if err != nil {
		return nil, err
	}

	group, err := l.svcCtx.GroupsLogic.CreateGroup(l.ctx, business.CreateGroupRequest{
		CreatorUserID: userID,
		Name:          req.Name,
		Description:   req.Description,
		MemberUserIDs: req.MemberUserIDs,
	})
	if err != nil {
		return nil, err
	}
	return groupResp(group), nil
}
