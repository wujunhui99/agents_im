package rpc

import (
	"context"

	"github.com/wujunhui99/agents_im/internal/logic"
)

type FriendsServer struct {
	logic *logic.FriendsLogic
}

func NewFriendsServer(logic *logic.FriendsLogic) *FriendsServer {
	return &FriendsServer{logic: logic}
}

func (s *FriendsServer) AddFriend(ctx context.Context, req logic.AddFriendRequest) (logic.AddFriendResponse, error) {
	return s.logic.AddFriend(ctx, req)
}

func (s *FriendsServer) DeleteFriend(ctx context.Context, req logic.DeleteFriendRequest) (logic.DeleteFriendResponse, error) {
	return s.logic.DeleteFriend(ctx, req)
}

func (s *FriendsServer) ListFriends(ctx context.Context, req logic.ListFriendsRequest) (logic.ListFriendsResponse, error) {
	return s.logic.ListFriends(ctx, req)
}

func (s *FriendsServer) GetFriendship(ctx context.Context, req logic.GetFriendshipRequest) (logic.GetFriendshipResponse, error) {
	return s.logic.GetFriendship(ctx, req)
}
