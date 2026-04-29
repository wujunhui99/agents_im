package logic

import (
	business "github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/proto/friendspb"
)

func toFriendship(view business.FriendshipView) *friendspb.Friendship {
	return &friendspb.Friendship{
		UserId:    view.UserID,
		FriendId:  view.FriendID,
		Status:    view.Status,
		IsFriend:  view.IsFriend,
		CreatedAt: view.CreatedAt,
		UpdatedAt: view.UpdatedAt,
	}
}
