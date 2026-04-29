package rpc

import (
	"context"

	"github.com/wujunhui99/agents_im/internal/logic"
)

type UserServer struct {
	logic *logic.UserLogic
}

func NewUserServer(logic *logic.UserLogic) *UserServer {
	return &UserServer{logic: logic}
}

func (s *UserServer) CreateUser(ctx context.Context, req logic.CreateUserRequest) (logic.UserProfile, error) {
	return s.logic.CreateUser(ctx, req)
}

func (s *UserServer) GetUserByIdentifier(ctx context.Context, req logic.GetUserByIdentifierRequest) (logic.UserProfile, error) {
	return s.logic.GetUserByIdentifier(ctx, req)
}

func (s *UserServer) ExistsByIdentifier(ctx context.Context, req logic.ExistsByIdentifierRequest) (logic.ExistsByIdentifierResponse, error) {
	return s.logic.ExistsByIdentifier(ctx, req)
}

func (s *UserServer) GetUserByID(ctx context.Context, req logic.GetUserByIDRequest) (logic.UserProfile, error) {
	return s.logic.GetUserByID(ctx, req)
}

func (s *UserServer) UpdateUserProfile(ctx context.Context, req logic.UpdateUserProfileRequest) (logic.UserProfile, error) {
	return s.logic.UpdateUserProfile(ctx, req)
}
