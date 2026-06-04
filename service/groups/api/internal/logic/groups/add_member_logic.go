package groups

import (
	"context"
	"strings"

	"github.com/wujunhui99/agents_im/pkg/ctxuser"
	"github.com/wujunhui99/agents_im/service/groups/api/internal/svc"
	"github.com/wujunhui99/agents_im/service/groups/api/internal/types"
	groupspb "github.com/wujunhui99/agents_im/service/groups/rpc/groups"
	"github.com/zeromicro/go-zero/core/logx"
)

type AddMemberLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAddMemberLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AddMemberLogic {
	return &AddMemberLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AddMemberLogic) AddMember(req *types.AddMemberReq) (*types.MemberResp, error) {
	userID, err := ctxuser.UserID(l.ctx)
	if err != nil {
		return nil, err
	}
	// 建成员前校验 operator 与目标用户存在（目标为空时 rpc 默认取 operator）。
	ids := []string{userID}
	if strings.TrimSpace(req.UserID) != "" {
		ids = append(ids, req.UserID)
	}
	if err := ensureUsersExist(l.ctx, l.svcCtx, ids...); err != nil {
		return nil, err
	}
	resp, err := l.svcCtx.GroupsRPC.AddMember(l.ctx, &groupspb.AddMemberRequest{GroupId: req.GroupID, OperatorUserId: userID, UserId: req.UserID})
	if err != nil {
		return nil, apiError(err)
	}
	member, err := hydrateMember(l.ctx, l.svcCtx, resp.GetMember())
	if err != nil {
		return nil, err
	}
	return memberRespWith(member, resp.GetAlreadyMember()), nil
}
