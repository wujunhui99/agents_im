package model

import "github.com/zeromicro/go-zero/core/stores/sqlx"

var ErrNotFound = sqlx.ErrNotFound

// accounts.account_type 的数据库整型取值（smallint），单一来源：model 写 SQL、
// logic 做字符串映射都引用此处。与 db/migrations/001 一致。
const (
	AccountTypeAdmin int64 = 0
	AccountTypeUser  int64 = 1
	AccountTypeAgent int64 = 2
	AccountTypeTest  int64 = 3
)

// profiles.gender 的数据库整型取值（smallint），单一来源。与 db/migrations/001 一致。
const (
	GenderUnknown int64 = 0
	GenderMale    int64 = 1
	GenderFemale  int64 = 2
	GenderOther   int64 = 3
)
