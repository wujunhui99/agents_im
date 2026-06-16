package model

import (
	"context"
	"database/sql"
	"strconv"
	"strings"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ AccountsModel = (*customAccountsModel)(nil)

// AccountProfile 是 accounts ⋈ profiles 的组合读模型（一条 join 取回身份+资料）。
// birth_date 经 coalesce 取成字符串（NULL 取空串），避免 date 类型在传输层来回转换。
type AccountProfile struct {
	AccountID        string       `db:"account_id"`
	Identifier       string       `db:"identifier"`
	AccountType      int64        `db:"account_type"`
	EmailNormalized  string       `db:"email_normalized"`
	EmailVerifiedAt  sql.NullTime `db:"email_verified_at"`
	AccountCreatedAt time.Time    `db:"account_created_at"`
	AccountUpdatedAt time.Time    `db:"account_updated_at"`
	DisplayName      string       `db:"display_name"`
	Name             string       `db:"name"`
	Gender           int64        `db:"gender"`
	BirthDate        string       `db:"birth_date"`
	Region           string       `db:"region"`
	AvatarMediaID    string       `db:"avatar_media_id"`
	AvatarURL        string       `db:"avatar_url"`
	ProfileCreatedAt time.Time    `db:"profile_created_at"`
	ProfileUpdatedAt time.Time    `db:"profile_updated_at"`
}

type (
	// AccountsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customAccountsModel.
	AccountsModel interface {
		accountsModel
		// WithSession 返回绑定到给定事务 session 的 model，供 Logic 层在事务内复用。
		WithSession(session sqlx.Session) AccountsModel
		// Transact 暴露事务入口，事务边界由 Logic 层控制（Model 不自行编排业务事务）。
		Transact(ctx context.Context, fn func(ctx context.Context, session sqlx.Session) error) error

		// FindAccountProfileByID 按 account_id 取回 account⋈profile；不存在返回 ErrNotFound。
		FindAccountProfileByID(ctx context.Context, accountID string) (*AccountProfile, error)
		// FindAccountProfileByIdentifier 按 identifier 取回 account⋈profile；不存在返回 ErrNotFound。
		FindAccountProfileByIdentifier(ctx context.Context, identifier string) (*AccountProfile, error)
		// ListAccountProfilesByIDs 批量取 account⋈profile（去重，WHERE account_id IN (...)）；
		// 不存在的 id 静默跳过，返回找到的子集（不保证顺序）。
		ListAccountProfilesByIDs(ctx context.Context, accountIDs []string) ([]*AccountProfile, error)
		// ExistsByIdentifier 报告 identifier 是否已存在。
		ExistsByIdentifier(ctx context.Context, identifier string) (bool, error)
		// SearchAccountProfiles 按 query 模糊搜 account⋈profile（account_id/identifier/
		// display_name/name LIKE，大小写不敏感）；query 空则返回最近创建的 limit 条。管理后台用。
		SearchAccountProfiles(ctx context.Context, query string, limit int) ([]*AccountProfile, error)
		// CountAccounts 统计账号总数。
		CountAccounts(ctx context.Context) (int64, error)
	}

	customAccountsModel struct {
		*defaultAccountsModel
	}
)

const accountProfileSelectPrefix = `
select
  a.account_id, a.identifier, a.account_type, a.email_normalized, a.email_verified_at,
  a.created_at as account_created_at, a.updated_at as account_updated_at,
  p.display_name, p.name, p.gender, coalesce(p.birth_date::text, '') as birth_date, p.region, p.avatar_media_id, p.avatar_url,
  p.created_at as profile_created_at, p.updated_at as profile_updated_at
from accounts a
join profiles p on p.account_id = a.account_id
`

// NewAccountsModel returns a model for the database table.
func NewAccountsModel(conn sqlx.SqlConn) AccountsModel {
	return &customAccountsModel{
		defaultAccountsModel: newAccountsModel(conn),
	}
}

func (m *customAccountsModel) WithSession(session sqlx.Session) AccountsModel {
	return NewAccountsModel(sqlx.NewSqlConnFromSession(session))
}

func (m *customAccountsModel) Transact(ctx context.Context, fn func(ctx context.Context, session sqlx.Session) error) error {
	return m.conn.TransactCtx(ctx, fn)
}

func (m *customAccountsModel) FindAccountProfileByID(ctx context.Context, accountID string) (*AccountProfile, error) {
	var resp AccountProfile
	err := m.conn.QueryRowCtx(ctx, &resp, accountProfileSelectPrefix+"where a.account_id = $1", accountID)
	switch err {
	case nil:
		return &resp, nil
	case sqlx.ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}

func (m *customAccountsModel) FindAccountProfileByIdentifier(ctx context.Context, identifier string) (*AccountProfile, error) {
	var resp AccountProfile
	err := m.conn.QueryRowCtx(ctx, &resp, accountProfileSelectPrefix+"where a.identifier = $1", identifier)
	switch err {
	case nil:
		return &resp, nil
	case sqlx.ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}

func (m *customAccountsModel) ListAccountProfilesByIDs(ctx context.Context, accountIDs []string) ([]*AccountProfile, error) {
	placeholders := make([]string, 0, len(accountIDs))
	args := make([]any, 0, len(accountIDs))
	seen := make(map[string]struct{}, len(accountIDs))
	for _, id := range accountIDs {
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		args = append(args, id)
		placeholders = append(placeholders, "$"+strconv.Itoa(len(args)))
	}
	if len(args) == 0 {
		return nil, nil
	}

	query := accountProfileSelectPrefix + "where a.account_id in (" + strings.Join(placeholders, ",") + ")"
	var resp []*AccountProfile
	if err := m.conn.QueryRowsCtx(ctx, &resp, query, args...); err != nil {
		return nil, err
	}
	return resp, nil
}

func (m *customAccountsModel) ExistsByIdentifier(ctx context.Context, identifier string) (bool, error) {
	var exists bool
	err := m.conn.QueryRowCtx(ctx, &exists, "select exists(select 1 from accounts where identifier = $1)", identifier)
	return exists, err
}

// SearchAccountProfiles 复刻 internal/repository.SearchAccounts 的语义（行为对齐管理后台旧路径）。
func (m *customAccountsModel) SearchAccountProfiles(ctx context.Context, query string, limit int) ([]*AccountProfile, error) {
	query = strings.ToLower(strings.TrimSpace(query))
	like := "%" + query + "%"
	sql := accountProfileSelectPrefix + `where $1 = ''
   or lower(a.account_id) like $2
   or lower(a.identifier) like $2
   or lower(p.display_name) like $2
   or lower(p.name) like $2
order by a.created_at desc, a.account_id asc
limit $3`
	var resp []*AccountProfile
	if err := m.conn.QueryRowsCtx(ctx, &resp, sql, query, like, limit); err != nil {
		return nil, err
	}
	return resp, nil
}

func (m *customAccountsModel) CountAccounts(ctx context.Context) (int64, error) {
	var count int64
	err := m.conn.QueryRowCtx(ctx, &count, "select count(*) from accounts")
	return count, err
}
