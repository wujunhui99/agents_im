package groups

import (
	"context"

	"github.com/wujunhui99/agents_im/pkg/ctxuser"
	"github.com/wujunhui99/agents_im/service/groups/api/internal/svc"
	"github.com/wujunhui99/agents_im/service/groups/api/internal/types"
	groupspb "github.com/wujunhui99/agents_im/service/groups/rpc/groups"
	"github.com/zeromicro/go-zero/core/logx"
)

type CreateGroupLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateGroupLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateGroupLogic {
	return &CreateGroupLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *CreateGroupLogic) CreateGroup(req *types.CreateGroupReq) (*types.GroupResp, error) {
	userID, err := ctxuser.UserID(l.ctx)
	if err != nil {
		return nil, err
	}
	// 跨域用户存在性校验上移 BFF：建群前确认 creator 与所有成员存在。
	if err := ensureUsersExist(l.ctx, l.svcCtx, append([]string{userID}, req.MemberUserIDs...)...); err != nil {
		return nil, err
	}
	resp, err := l.svcCtx.GroupsRPC.CreateGroup(l.ctx, &groupspb.CreateGroupRequest{CreatorUserId: userID, Name: req.Name, Description: req.Description, MemberUserIds: req.MemberUserIDs})
	if err != nil {
		return nil, apiError(err)
	}
	return groupResp(resp.GetGroup()), nil
}
