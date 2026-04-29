package groups

import (
	"context"

	"github.com/wujunhui99/agents_im/internal/apperror"
	"github.com/wujunhui99/agents_im/internal/ctxuser"
	business "github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/svc"
	"github.com/wujunhui99/agents_im/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type AddMemberLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAddMemberLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AddMemberLogic {
	return &AddMemberLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AddMemberLogic) AddMember(req *types.AddMemberReq) (*types.MemberResp, error) {
	userID, err := ctxuser.UserID(l.ctx)
	if err != nil {
		return nil, err
	}

	result, err := l.svcCtx.GroupsLogic.AddMember(l.ctx, business.AddMemberRequest{
		GroupID:        req.GroupID,
		OperatorUserID: userID,
		UserID:         req.UserID,
	})
	if err != nil {
		return nil, err
	}
	return memberResp(result), nil
}

type CreateGroupLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateGroupLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateGroupLogic {
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
	})
	if err != nil {
		return nil, err
	}
	return groupResp(group), nil
}

type GetGroupLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetGroupLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetGroupLogic {
	return &GetGroupLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetGroupLogic) GetGroup(req *types.GetGroupReq) (*types.GroupResp, error) {
	group, err := l.svcCtx.GroupsLogic.GetGroup(l.ctx, business.GetGroupRequest{GroupID: req.GroupID})
	if err != nil {
		return nil, err
	}
	return groupResp(group), nil
}

type LeaveGroupLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewLeaveGroupLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LeaveGroupLogic {
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

type ListMembersLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListMembersLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListMembersLogic {
	return &ListMembersLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListMembersLogic) ListMembers(req *types.ListMembersReq) (*types.ListMembersResp, error) {
	result, err := l.svcCtx.GroupsLogic.ListMembers(l.ctx, business.ListMembersRequest{GroupID: req.GroupID})
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

func groupResp(group business.GroupInfo) *types.GroupResp {
	return &types.GroupResp{
		Code:    string(apperror.CodeOK),
		Message: "ok",
		Data: types.Group{
			GroupID:       group.GroupID,
			Name:          group.Name,
			Description:   group.Description,
			CreatorUserID: group.CreatorUserID,
			CreatedAt:     group.CreatedAt,
			UpdatedAt:     group.UpdatedAt,
		},
	}
}

func memberResp(member business.MemberResponse) *types.MemberResp {
	return &types.MemberResp{
		Code:    string(apperror.CodeOK),
		Message: "ok",
		Data: types.MemberData{
			Member:        toGroupMember(member.Member),
			AlreadyMember: member.AlreadyMember,
		},
	}
}

func toGroupMember(member business.GroupMemberInfo) types.GroupMember {
	return types.GroupMember{
		GroupID:  member.GroupID,
		UserID:   member.UserID,
		State:    member.State,
		JoinedAt: member.JoinedAt,
		LeftAt:   member.LeftAt,
	}
}
