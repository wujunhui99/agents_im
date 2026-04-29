package rpc

import (
	"context"

	"github.com/wujunhui99/agents_im/internal/logic"
)

type GroupsServer struct {
	logic *logic.GroupsLogic
}

func NewGroupsServer(logic *logic.GroupsLogic) *GroupsServer {
	return &GroupsServer{logic: logic}
}

func (s *GroupsServer) CreateGroup(ctx context.Context, req logic.CreateGroupRequest) (logic.GroupInfo, error) {
	return s.logic.CreateGroup(ctx, req)
}

func (s *GroupsServer) GetGroup(ctx context.Context, req logic.GetGroupRequest) (logic.GroupInfo, error) {
	return s.logic.GetGroup(ctx, req)
}

func (s *GroupsServer) AddMember(ctx context.Context, req logic.AddMemberRequest) (logic.MemberResponse, error) {
	return s.logic.AddMember(ctx, req)
}

func (s *GroupsServer) JoinGroup(ctx context.Context, req logic.JoinGroupRequest) (logic.MemberResponse, error) {
	return s.logic.JoinGroup(ctx, req)
}

func (s *GroupsServer) LeaveGroup(ctx context.Context, req logic.LeaveGroupRequest) (logic.MemberResponse, error) {
	return s.logic.LeaveGroup(ctx, req)
}

func (s *GroupsServer) ListMembers(ctx context.Context, req logic.ListMembersRequest) (logic.ListMembersResponse, error) {
	return s.logic.ListMembers(ctx, req)
}
