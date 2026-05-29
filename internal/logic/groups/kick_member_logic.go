package groups

import (
	"context"

	"github.com/wujunhui99/agents_im/pkg/ctxuser"
	business "github.com/wujunhui99/agents_im/internal/logic"
	groupssvc "github.com/wujunhui99/agents_im/internal/servicecontext/groups"
	"github.com/wujunhui99/agents_im/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type KickMemberLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *groupssvc.ServiceContext
}

func NewKickMemberLogic(ctx context.Context, svcCtx *groupssvc.ServiceContext) *KickMemberLogic {
	return &KickMemberLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *KickMemberLogic) KickMember(req *types.KickMemberReq) (*types.MemberResp, error) {
	userID, err := ctxuser.UserID(l.ctx)
	if err != nil {
		return nil, err
	}

	result, err := l.svcCtx.GroupsLogic.KickMember(l.ctx, business.KickMemberRequest{
		GroupID:        req.GroupID,
		OperatorUserID: userID,
		UserID:         req.UserID,
	})
	if err != nil {
		return nil, err
	}
	return memberResp(result), nil
}
