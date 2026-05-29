package groups

import (
	"context"

	"github.com/wujunhui99/agents_im/pkg/ctxuser"
	"github.com/wujunhui99/agents_im/service/groups/api/internal/svc"
	"github.com/wujunhui99/agents_im/service/groups/api/internal/types"
	groupspb "github.com/wujunhui99/agents_im/service/groups/rpc/groups"
	"github.com/zeromicro/go-zero/core/logx"
)

type KickMemberLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewKickMemberLogic(ctx context.Context, svcCtx *svc.ServiceContext) *KickMemberLogic {
	return &KickMemberLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *KickMemberLogic) KickMember(req *types.KickMemberReq) (*types.MemberResp, error) {
	userID, err := ctxuser.UserID(l.ctx)
	if err != nil {
		return nil, err
	}
	resp, err := l.svcCtx.GroupsRPC.KickMember(l.ctx, &groupspb.KickMemberRequest{GroupId: req.GroupID, OperatorUserId: userID, UserId: req.UserID})
	if err != nil {
		return nil, apiError(err)
	}
	return memberResp(resp), nil
}
