package model

import (
	"context"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ AuthEmailVerificationTokensModel = (*customAuthEmailVerificationTokensModel)(nil)

type (
	// AuthEmailVerificationTokensModel is an interface to be customized, add more methods here,
	// and implement the added methods in customAuthEmailVerificationTokensModel.
	AuthEmailVerificationTokensModel interface {
		authEmailVerificationTokensModel
		// Latest 取 (purpose,email) 最近一条 token；无记录返回 ErrNotFound。
		Latest(ctx context.Context, purpose int64, emailNormalized string) (*AuthEmailVerificationTokens, error)
		// SupersedeAndInsert 在一个事务内把 (purpose,email) 现存未消费 token 标记为已消费，
		// 再插入新 token（同一邮箱同一用途只保留一条有效验证码）。
		SupersedeAndInsert(ctx context.Context, data *AuthEmailVerificationTokens) error
		// IncrementAttempts 对 id 的 attempt_count +1，返回自增后的尝试次数。
		IncrementAttempts(ctx context.Context, id string, now time.Time) (int64, error)
		// Consume 把未消费且未过期的 token 标记为已消费（attempt_count 也 +1），返回 consumed_at；
		// 不满足条件返回 ErrNotFound。
		Consume(ctx context.Context, id string, now time.Time) (time.Time, error)
	}

	customAuthEmailVerificationTokensModel struct {
		*defaultAuthEmailVerificationTokensModel
	}
)

// NewAuthEmailVerificationTokensModel returns a model for the database table.
func NewAuthEmailVerificationTokensModel(conn sqlx.SqlConn) AuthEmailVerificationTokensModel {
	return &customAuthEmailVerificationTokensModel{
		defaultAuthEmailVerificationTokensModel: newAuthEmailVerificationTokensModel(conn),
	}
}

func (m *customAuthEmailVerificationTokensModel) Latest(ctx context.Context, purpose int64, emailNormalized string) (*AuthEmailVerificationTokens, error) {
	var row AuthEmailVerificationTokens
	err := m.conn.QueryRowCtx(ctx, &row, `
select `+authEmailVerificationTokensRows+`
from `+m.table+`
where purpose = $1 and email_normalized = $2
order by created_at desc, id desc
limit 1`, purpose, emailNormalized)
	switch err {
	case nil:
		return &row, nil
	case sqlx.ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}

func (m *customAuthEmailVerificationTokensModel) SupersedeAndInsert(ctx context.Context, data *AuthEmailVerificationTokens) error {
	return m.conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		now := data.CreatedAt.UTC()
		if data.CreatedAt.IsZero() {
			now = time.Now().UTC()
		}
		if _, err := session.ExecCtx(ctx, `
update `+m.table+`
set consumed_at = $3, updated_at = $3
where purpose = $1 and email_normalized = $2 and consumed_at is null`,
			data.Purpose, data.EmailNormalized, now); err != nil {
			return err
		}
		_, err := session.ExecCtx(ctx, `
insert into `+m.table+` (id, purpose, email_normalized, code_hash, code_hash_algo, expires_at, attempt_count, last_sent_at)
values ($1, $2, $3, $4, $5, $6, $7, $8)`,
			data.Id, data.Purpose, data.EmailNormalized, data.CodeHash, data.CodeHashAlgo,
			data.ExpiresAt.UTC(), data.AttemptCount, data.LastSentAt.UTC())
		return err
	})
}

func (m *customAuthEmailVerificationTokensModel) IncrementAttempts(ctx context.Context, id string, now time.Time) (int64, error) {
	var attemptCount int64
	err := m.conn.QueryRowCtx(ctx, &attemptCount, `
update `+m.table+`
set attempt_count = attempt_count + 1, updated_at = $2
where id = $1
returning attempt_count`, id, now.UTC())
	switch err {
	case nil:
		return attemptCount, nil
	case sqlx.ErrNotFound:
		return 0, ErrNotFound
	default:
		return 0, err
	}
}

func (m *customAuthEmailVerificationTokensModel) Consume(ctx context.Context, id string, now time.Time) (time.Time, error) {
	var consumedAt time.Time
	err := m.conn.QueryRowCtx(ctx, &consumedAt, `
update `+m.table+`
set consumed_at = $2, attempt_count = attempt_count + 1, updated_at = $2
where id = $1 and consumed_at is null and expires_at > $2
returning consumed_at`, id, now.UTC())
	switch err {
	case nil:
		return consumedAt, nil
	case sqlx.ErrNotFound:
		return time.Time{}, ErrNotFound
	default:
		return time.Time{}, err
	}
}
