package model

import (
	"context"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// accounts.account_type 的 DB 整型取值（smallint）。单一来源在 user 域
// `service/user/rpc/internal/model/vars.go`；这里是跨域鉴权读所需的本地副本。
const (
	AccountTypeDBAdmin int64 = 0
	AccountTypeDBTest  int64 = 3
)

// AccountsGuardModel 是跨域鉴权读（keystone 例外）：accounts 是 user 域的表，
// 这里只读 account_type，用于 EnsureTestCredential 的访问控制（防止该 rpc 被误用
// 于覆盖非 test 账户的密码）。参照 media #433「跨域鉴权读暂留 owner rpc」例外；
// 待 auth 域整体重构（退役 internal/auth）时统一处理。
type AccountsGuardModel interface {
	// FindAccountTypeByID 返回 account_id 对应的 account_type；不存在返回 ErrNotFound。
	FindAccountTypeByID(ctx context.Context, accountID string) (int64, error)
}

type defaultAccountsGuardModel struct {
	conn sqlx.SqlConn
}

func NewAccountsGuardModel(conn sqlx.SqlConn) AccountsGuardModel {
	return &defaultAccountsGuardModel{conn: conn}
}

func (m *defaultAccountsGuardModel) FindAccountTypeByID(ctx context.Context, accountID string) (int64, error) {
	var accountType int64
	err := m.conn.QueryRowCtx(ctx, &accountType, `select account_type from "public"."accounts" where account_id = $1 limit 1`, accountID)
	if err != nil {
		return 0, err
	}
	return accountType, nil
}
