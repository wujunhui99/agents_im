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

type AddMemberLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *groupssvc.ServiceContext
}

func NewAddMemberLogic(ctx context.Context, svcCtx *groupssvc.ServiceContext) *AddMemberLogic {
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

type ListGroupsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *groupssvc.ServiceContext
}

func NewListGroupsLogic(ctx context.Context, svcCtx *groupssvc.ServiceContext) *ListGroupsLogic {
	return &ListGroupsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListGroupsLogic) ListGroups(_ *types.ListGroupsReq) (*types.ListGroupsResp, error) {
	userID, err := ctxuser.UserID(l.ctx)
	if err != nil {
		return nil, err
	}

	result, err := l.svcCtx.GroupsLogic.ListGroups(l.ctx, business.ListGroupsRequest{UserID: userID})
	if err != nil {
		return nil, err
	}
	groups := make([]types.Group, 0, len(result.Groups))
	for _, group := range result.Groups {
		groups = append(groups, toGroup(group))
	}
	return &types.ListGroupsResp{
		Code:    string(apperror.CodeOK),
		Message: "ok",
		Data:    types.ListGroupsData{Groups: groups},
	}, nil
}

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

func groupResp(group business.GroupInfo) *types.GroupResp {
	return &types.GroupResp{
		Code:    string(apperror.CodeOK),
		Message: "ok",
		Data:    toGroup(group),
	}
}

func toGroup(group business.GroupInfo) types.Group {
	return types.Group{
		GroupID:         group.GroupID,
		Name:            group.Name,
		Description:     group.Description,
		Announcement:    group.Announcement,
		AvatarMediaID:   group.AvatarMediaID,
		AvatarURL:       group.AvatarURL,
		CreatorUserID:   group.CreatorUserID,
		CurrentUserRole: group.CurrentUserRole,
		CreatedAt:       group.CreatedAt,
		UpdatedAt:       group.UpdatedAt,
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
		GroupID:       member.GroupID,
		UserID:        member.UserID,
		Role:          member.Role,
		State:         member.State,
		JoinedAt:      member.JoinedAt,
		LeftAt:        member.LeftAt,
		Identifier:    member.Identifier,
		DisplayName:   member.DisplayName,
		Name:          member.Name,
		AvatarMediaID: member.AvatarMediaID,
		AvatarURL:     member.AvatarURL,
	}
}
