package groups

import (
	"context"

	"github.com/wujunhui99/agents_im/internal/apperror"
	"github.com/wujunhui99/agents_im/internal/ctxuser"
	business "github.com/wujunhui99/agents_im/internal/logic"
	groupssvc "github.com/wujunhui99/agents_im/internal/servicecontext/groups"
	"github.com/wujunhui99/agents_im/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type ListMembersLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *groupssvc.ServiceContext
}

func NewListMembersLogic(ctx context.Context, svcCtx *groupssvc.ServiceContext) *ListMembersLogic {
	return &ListMembersLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListMembersLogic) ListMembers(req *types.ListMembersReq) (*types.ListMembersResp, error) {
	userID, err := ctxuser.UserID(l.ctx)
	if err != nil {
		return nil, err
	}

	result, err := l.svcCtx.GroupsLogic.ListMembers(l.ctx, business.ListMembersRequest{
		GroupID:         req.GroupID,
		RequesterUserID: userID,
	})
	if err != nil {
		return nil, err
	}

	members := make([]types.GroupMember, 0, len(result.Members))
	for _, member := range result.Members {
		members = append(members, toGroupMember(member))
	}
	return &types.ListMembersResp{
		Code:    string(apperror.CodeOK),
		Message: "ok",
		Data: types.ListMembersData{
			GroupID: result.GroupID,
			Members: members,
		},
	}, nil
}
