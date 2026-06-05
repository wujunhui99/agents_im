package logic

import (
	business "github.com/wujunhui99/agents_im/internal/logic"
	userpb "github.com/wujunhui99/agents_im/service/user/rpc/user"
)

func toUserResponse(profile business.UserProfile) *userpb.UserResponse {
	return &userpb.UserResponse{User: toUserEntity(profile)}
}

func toUserEntity(profile business.UserProfile) *userpb.UserEntity {
	return &userpb.UserEntity{
		UserId:        profile.UserID,
		Identifier:    profile.Identifier,
		DisplayName:   profile.DisplayName,
		Name:          profile.Name,
		Gender:        profile.Gender,
		BirthDate:     profile.BirthDate,
		Region:        profile.Region,
		AccountType:   profile.AccountType,
		AvatarMediaId: profile.AvatarMediaID,
		Email:         profile.Email,
		AvatarUrl:     profile.AvatarURL,
		CreatedAt:     profile.CreatedAt,
		UpdatedAt:     profile.UpdatedAt,
	}
}
