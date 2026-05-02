package logic

import (
	business "github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/proto/userpb"
)

func toUserResponse(profile business.UserProfile) *userpb.UserResponse {
	return &userpb.UserResponse{
		User: &userpb.User{
			UserId:        profile.UserID,
			Identifier:    profile.Identifier,
			DisplayName:   profile.DisplayName,
			Name:          profile.Name,
			Gender:        profile.Gender,
			Age:           profile.Age,
			Region:        profile.Region,
			AccountType:   profile.AccountType,
			AvatarMediaId: profile.AvatarMediaID,
			CreatedAt:     profile.CreatedAt,
			UpdatedAt:     profile.UpdatedAt,
		},
	}
}
