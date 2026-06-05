package logic

import (
	"time"

	friends "github.com/wujunhui99/agents_im/service/friends/rpc/friends"
	"github.com/wujunhui99/agents_im/service/friends/rpc/internal/model"
)

// toFriendship 把一条 friendships 行转为 rpc Friendship。
// 跨域好友资料（FriendProfile）由 api(BFF) 聚合 user-rpc 补全，rpc 不再返回。
func toFriendship(row *model.Friendships) *friends.Friendship {
	if row == nil {
		return nil
	}
	return friendshipView(row.AccountId, row.FriendAccountId, row.Status, row.CreatedAt, row.UpdatedAt)
}

// friendshipView 构造一条 Friendship 视图。
func friendshipView(userID, friendID string, status int64, createdAt, updatedAt time.Time) *friends.Friendship {
	return &friends.Friendship{
		UserId:    userID,
		FriendId:  friendID,
		Status:    statusToString(status),
		IsFriend:  status == model.FriendshipStatusAccepted,
		CreatedAt: formatTime(createdAt),
		UpdatedAt: formatTime(updatedAt),
	}
}

// noneFriendship 构造一条逻辑“无关系”视图（GetFriendship 未命中任何行时返回）。
func noneFriendship(userID, friendID string) *friends.Friendship {
	return &friends.Friendship{
		UserId:   userID,
		FriendId: friendID,
		Status:   friendshipStatusNone,
		IsFriend: false,
	}
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
