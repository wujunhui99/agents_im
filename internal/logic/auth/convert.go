package auth

import (
	"github.com/wujunhui99/agents_im/internal/apperror"
	business "github.com/wujunhui99/agents_im/internal/auth/logic"
	"github.com/wujunhui99/agents_im/internal/types"
)

func authResp(result business.AuthResponse) *types.AuthResp {
	return &types.AuthResp{
		Code:    string(apperror.CodeOK),
		Message: "ok",
		Data: types.AuthData{
			UserID:        result.UserID,
			Identifier:    result.Identifier,
			Email:         result.Email,
			DisplayName:   result.DisplayName,
			Name:          result.Name,
			Gender:        result.Gender,
			BirthDate:     result.BirthDate,
			Region:        result.Region,
			AccountType:   result.AccountType,
			AvatarMediaID: result.AvatarMediaID,
			AvatarURL:     result.AvatarURL,
			Token:         result.Token,
			ExpiresAt:     result.ExpiresAt,
		},
	}
}
