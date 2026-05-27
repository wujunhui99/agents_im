package logic

import (
	business "github.com/wujunhui99/agents_im/internal/logic"
	friends "github.com/wujunhui99/agents_im/service/friends/rpc/friends"
)

func toFriendship(view business.FriendshipView) *friends.Friendship {
	friendship := &friends.Friendship{
		UserId:    view.UserID,
		FriendId:  view.FriendID,
		Status:    view.Status,
		IsFriend:  view.IsFriend,
		CreatedAt: view.CreatedAt,
		UpdatedAt: view.UpdatedAt,
	}
	if view.Friend != nil {
		friendship.Friend = toFriendProfile(*view.Friend)
	}
	return friendship
}

func toFriendProfile(profile business.UserProfile) *friends.FriendProfile {
	return &friends.FriendProfile{
		UserId:        profile.UserID,
		Identifier:    profile.Identifier,
		DisplayName:   profile.DisplayName,
		Name:          profile.Name,
		Gender:        profile.Gender,
		BirthDate:     profile.BirthDate,
		Region:        profile.Region,
		AccountType:   profile.AccountType,
		AvatarMediaId: profile.AvatarMediaID,
		AvatarUrl:     profile.AvatarURL,
		CreatedAt:     profile.CreatedAt,
		UpdatedAt:     profile.UpdatedAt,
	}
}
