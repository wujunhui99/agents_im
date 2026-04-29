package logic

import (
	business "github.com/wujunhui99/agents_im/internal/auth/logic"
	"github.com/wujunhui99/agents_im/proto/authpb"
)

func toAuthResponse(result business.AuthResponse) *authpb.AuthResponse {
	return &authpb.AuthResponse{
		UserId:     result.UserID,
		Identifier: result.Identifier,
		Token:      result.Token,
		ExpiresAt:  result.ExpiresAt,
	}
}

func toValidateTokenResponse(result business.ValidateTokenResponse) *authpb.ValidateTokenResponse {
	return &authpb.ValidateTokenResponse{
		Valid:      result.Valid,
		UserId:     result.UserID,
		Identifier: result.Identifier,
		ExpiresAt:  result.ExpiresAt,
	}
}
