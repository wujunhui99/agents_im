package model

import "github.com/zeromicro/go-zero/core/stores/sqlx"

var ErrNotFound = sqlx.ErrNotFound

// friendships.status 的数据库整型取值（smallint），单一来源：
// model 写 SQL、logic 做字符串映射都引用此处。与 db/migrations/001 一致
// （-- friendships.status: 1=pending, 2=accepted, 3=rejected, 4=deleted）。
const (
	FriendshipStatusPending  int64 = 1
	FriendshipStatusAccepted int64 = 2
	FriendshipStatusRejected int64 = 3
	FriendshipStatusDeleted  int64 = 4
)
