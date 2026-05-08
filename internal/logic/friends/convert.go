package friends

import (
	"context"

	business "github.com/wujunhui99/agents_im/internal/logic"
	friendssvc "github.com/wujunhui99/agents_im/internal/servicecontext/friends"
	"github.com/wujunhui99/agents_im/internal/types"
)

func toFriendship(ctx context.Context, svcCtx *friendssvc.ServiceContext, friendship business.FriendshipView) (types.Friendship, error) {
	view := types.Friendship{
		UserID:    friendship.UserID,
		FriendID:  friendship.FriendID,
		Status:    friendship.Status,
		IsFriend:  friendship.IsFriend,
		CreatedAt: friendship.CreatedAt,
		UpdatedAt: friendship.UpdatedAt,
	}
	if friendship.Friend != nil {
		profile, err := toFriendProfile(ctx, svcCtx, *friendship.Friend)
		if err != nil {
			return types.Friendship{}, err
		}
		view.Friend = profile
	}
	return view, nil
}

func toFriendProfile(ctx context.Context, svcCtx *friendssvc.ServiceContext, profile business.UserProfile) (types.FriendProfile, error) {
	return types.FriendProfile{
		UserID:        profile.UserID,
		Identifier:    profile.Identifier,
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
	}, nil
}
