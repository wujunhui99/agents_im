package user

import (
	"context"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	business "github.com/wujunhui99/agents_im/internal/logic"
	usersvc "github.com/wujunhui99/agents_im/internal/servicecontext/user"
	"github.com/wujunhui99/agents_im/internal/types"
)

func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func userResp(profile business.UserProfile) *types.UserResp {
	return userRespWithAvatarFields(profile)
}

func userRespWithAvatar(ctx context.Context, svcCtx *usersvc.ServiceContext, profile business.UserProfile) (*types.UserResp, error) {
	return userRespWithAvatarFields(profile), nil
}

func userRespWithAvatarFields(profile business.UserProfile) *types.UserResp {
	return &types.UserResp{
		Code:    string(apperror.CodeOK),
		Message: "ok",
		Data: types.User{
			UserID:        profile.UserID,
			Identifier:    profile.Identifier,
			Email:         profile.Email,
			DisplayName:   profile.DisplayName,
			Name:          profile.Name,
			Gender:        profile.Gender,
			BirthDate:     profile.BirthDate,
			Region:        profile.Region,
			AccountType:   profile.AccountType,
			AvatarMediaID: profile.AvatarMediaID,
			AvatarURL:     profile.AvatarURL,
			CreatedAt:     profile.CreatedAt,
			UpdatedAt:     profile.UpdatedAt,
		},
	}
}
