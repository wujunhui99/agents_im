package model

import "github.com/zeromicro/go-zero/core/stores/sqlx"

var ErrNotFound = sqlx.ErrNotFound

// group_members.role / .status 的数据库整型取值（smallint），单一来源：
// model 写 SQL、logic 做字符串映射都引用此处。与 db/migrations 一致。
const (
	MemberRoleOwner  int64 = 1
	MemberRoleMember int64 = 2
	MemberRoleAdmin  int64 = 3

	MemberStatusActive int64 = 1
	MemberStatusLeft   int64 = 2
)
