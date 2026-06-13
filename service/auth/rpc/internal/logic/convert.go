package logic

import (
	business "github.com/wujunhui99/agents_im/service/auth/core/logic"
	auth "github.com/wujunhui99/agents_im/service/auth/rpc/auth"
)

func toAuthResponse(result business.AuthResponse) *auth.AuthResponse {
	return &auth.AuthResponse{
		UserId:        result.UserID,
		Identifier:    result.Identifier,
		Email:         result.Email,
		DisplayName:   result.DisplayName,
		Name:          result.Name,
		Gender:        result.Gender,
		BirthDate:     result.BirthDate,
		Region:        result.Region,
		AccountType:   result.AccountType,
		AvatarMediaId: result.AvatarMediaID,
		AvatarUrl:     result.AvatarURL,
		Token:         result.Token,
		ExpiresAt:     result.ExpiresAt,
	}
}

func toValidateTokenResponse(result business.ValidateTokenResponse) *auth.ValidateTokenResponse {
	return &auth.ValidateTokenResponse{
		Valid:      result.Valid,
		UserId:     result.UserID,
		Identifier: result.Identifier,
		ExpiresAt:  result.ExpiresAt,
	}
}
