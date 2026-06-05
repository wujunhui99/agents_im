package logic

import (
	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/service/friends/rpc/internal/model"
)

// friendshipStatusNone 是逻辑上的“无关系”状态，不落库（DB status 取值见 model/vars.go 的 1..4）。
const friendshipStatusNone = "none"

// --- status 整型 -> 字符串映射（整型取值见 model/vars.go）---

func statusToString(status int64) string {
	switch status {
	case model.FriendshipStatusAccepted:
		return "accepted"
	case model.FriendshipStatusRejected:
		return "rejected"
	case model.FriendshipStatusDeleted:
		return "deleted"
	default:
		return "pending"
	}
}

// --- 输入校验（不做规范化：入参的清洗由客户端负责，后端只守完整性/防滥用底线）---

func validateRequiredID(value, field string) (string, error) {
	if value == "" {
		return "", apperror.InvalidArgument(field + " is required")
	}
	if len([]rune(value)) > 64 {
		return "", apperror.InvalidArgument(field + " must be 64 characters or fewer")
	}
	return value, nil
}

// validateFriendshipPair 校验 user_id / friend_id 均合法且互不相同。
func validateFriendshipPair(userID, friendID string) (string, string, error) {
	if _, err := validateRequiredID(userID, "user_id"); err != nil {
		return "", "", err
	}
	if _, err := validateRequiredID(friendID, "friend_id"); err != nil {
		return "", "", err
	}
	if userID == friendID {
		return "", "", apperror.InvalidArgument("user_id and friend_id must be different")
	}
	return userID, friendID, nil
}
