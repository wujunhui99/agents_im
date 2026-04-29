package rpc

import (
	"context"

	authlogic "github.com/wujunhui99/agents_im/internal/auth/logic"
)

type AuthServer struct {
	logic *authlogic.AuthLogic
}

func NewAuthServer(logic *authlogic.AuthLogic) *AuthServer {
	return &AuthServer{logic: logic}
}

func (s *AuthServer) Register(ctx context.Context, req authlogic.RegisterRequest) (authlogic.AuthResponse, error) {
	return s.logic.Register(ctx, req)
}

func (s *AuthServer) Login(ctx context.Context, req authlogic.LoginRequest) (authlogic.AuthResponse, error) {
	return s.logic.Login(ctx, req)
}

func (s *AuthServer) ValidateToken(ctx context.Context, req authlogic.ValidateTokenRequest) (authlogic.ValidateTokenResponse, error) {
	return s.logic.ValidateToken(ctx, req)
}

func (s *AuthServer) ParseToken(ctx context.Context, req authlogic.ValidateTokenRequest) (authlogic.ValidateTokenResponse, error) {
	return s.logic.ParseToken(ctx, req)
}
