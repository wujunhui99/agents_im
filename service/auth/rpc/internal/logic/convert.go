package logic

import (
	"context"

	"github.com/wujunhui99/agents_im/pkg/auth/token"
	auth "github.com/wujunhui99/agents_im/service/auth/rpc/auth"
	"github.com/wujunhui99/agents_im/service/auth/rpc/internal/svc"
	"github.com/wujunhui99/agents_im/service/user/rpc/userclient"
)

// issueToken 签发 JWT、写活跃会话，并用属主 user-rpc 返回的资料拼装 AuthResponse。
func issueToken(ctx context.Context, svcCtx *svc.ServiceContext, user *userclient.UserEntity, device string, loginIP string) (*auth.AuthResponse, error) {
	rawToken, claims, err := svcCtx.Tokens.Issue(user.GetUserId(), user.GetIdentifier(), device, loginIP)
	if err != nil {
		return nil, err
	}
	ttl := claims.ExpiresAt.Sub(claims.IssuedAt)
	if err := svcCtx.Sessions.SetActive(ctx, claims.UserID, claims.Device, claims.JTI, ttl); err != nil {
		return nil, err
	}
	return &auth.AuthResponse{
		UserId:        claims.UserID,
		Identifier:    claims.Identifier,
		Email:         user.GetEmail(),
		DisplayName:   user.GetDisplayName(),
		Name:          user.GetName(),
		Gender:        user.GetGender(),
		BirthDate:     user.GetBirthDate(),
		Region:        user.GetRegion(),
		AccountType:   user.GetAccountType(),
		AvatarMediaId: user.GetAvatarMediaId(),
		AvatarUrl:     user.GetAvatarUrl(),
		Token:         rawToken,
		ExpiresAt:     formatTime(claims.ExpiresAt),
	}, nil
}

func toValidateTokenResponse(claims token.Claims) *auth.ValidateTokenResponse {
	return &auth.ValidateTokenResponse{
		Valid:      true,
		UserId:     claims.UserID,
		Identifier: claims.Identifier,
		ExpiresAt:  formatTime(claims.ExpiresAt),
	}
}
