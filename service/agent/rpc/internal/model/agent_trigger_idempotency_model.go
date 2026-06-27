package model

import (
	"context"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ AgentTriggerIdempotencyModel = (*customAgentTriggerIdempotencyModel)(nil)

type (
	// AgentTriggerIdempotencyModel is an interface to be customized, add more methods here,
	// and implement the added methods in customAgentTriggerIdempotencyModel.
	AgentTriggerIdempotencyModel interface {
		agentTriggerIdempotencyModel
		withSession(session sqlx.Session) AgentTriggerIdempotencyModel

		// TryStart 抢占式占用一个触发 idempotency_key：新键 insert 占用；既有键仅当处于 running
		// 但已超过 runningTTLMillis（崩溃残留），或处于 failed（可重试）时才重置为 running 并抢占。
		// succeeded 终态或仍在 TTL 内的 running 返回 false（不重复触发）。整个判断在事务内完成。
		TryStart(ctx context.Context, data *AgentTriggerIdempotency, runningTTLMillis int64) (bool, error)
		// Finish 把仍处于 running 的触发推进到终态（succeeded/failed）。已是终态或不存在返回
		// false（幂等：终态不可被再次改写），由 Store 层翻译成 NotFound。
		Finish(ctx context.Context, idempotencyKey string, status string, responseServerMsgId string, errorMessage string) (bool, error)
	}

	customAgentTriggerIdempotencyModel struct {
		*defaultAgentTriggerIdempotencyModel
	}
)

// NewAgentTriggerIdempotencyModel returns a model for the database table.
func NewAgentTriggerIdempotencyModel(conn sqlx.SqlConn) AgentTriggerIdempotencyModel {
	return &customAgentTriggerIdempotencyModel{
		defaultAgentTriggerIdempotencyModel: newAgentTriggerIdempotencyModel(conn),
	}
}

func (m *customAgentTriggerIdempotencyModel) withSession(session sqlx.Session) AgentTriggerIdempotencyModel {
	return NewAgentTriggerIdempotencyModel(sqlx.NewSqlConnFromSession(session))
}

func (m *customAgentTriggerIdempotencyModel) TryStart(ctx context.Context, data *AgentTriggerIdempotency, runningTTLMillis int64) (bool, error) {
	if runningTTLMillis < 1 {
		runningTTLMillis = 1
	}
	var started bool
	err := m.conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		result, err := session.ExecCtx(ctx, "insert into "+m.table+` (
  idempotency_key, conversation_id, agent_account_id, trigger_server_msg_id, trigger_event_id, status
)
values ($1, $2, $3, $4, $5, $6)
on conflict (idempotency_key) do nothing
`, data.IdempotencyKey, data.ConversationId, data.AgentAccountId, data.TriggerServerMsgId, data.TriggerEventId, AgentTriggerStatusRunning)
		if err != nil {
			return err
		}
		rows, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if rows > 0 {
			started = true
			return nil
		}

		// 抢占谓词：status = failed（可重试，总是抢占）OR status = running 但已超 TTL（崩溃残留）。
		// succeeded 终态不在谓词内 → 不可抢占。$6 是写入的新 status（running）。
		result, err = session.ExecCtx(ctx, "update "+m.table+`
set conversation_id = $2,
    agent_account_id = $3,
    trigger_server_msg_id = $4,
    trigger_event_id = $5,
    status = $6,
    response_server_msg_id = '',
    error_message = '',
    updated_at = now()
where idempotency_key = $1
  and (
    status = $7
    or (status = $8 and updated_at <= now() - ($9 * interval '1 millisecond'))
  )
`, data.IdempotencyKey, data.ConversationId, data.AgentAccountId, data.TriggerServerMsgId,
			data.TriggerEventId, AgentTriggerStatusRunning, AgentTriggerStatusFailed, AgentTriggerStatusRunning, runningTTLMillis)
		if err != nil {
			return err
		}
		rows, err = result.RowsAffected()
		if err != nil {
			return err
		}
		started = rows > 0
		return nil
	})
	if err != nil {
		return false, err
	}
	return started, nil
}

func (m *customAgentTriggerIdempotencyModel) Finish(ctx context.Context, idempotencyKey string, status string, responseServerMsgId string, errorMessage string) (bool, error) {
	result, err := m.conn.ExecCtx(ctx, "update "+m.table+`
set status = $2,
    response_server_msg_id = $3,
    error_message = $4,
    updated_at = now()
where idempotency_key = $1 and status = $5
`, idempotencyKey, status, responseServerMsgId, errorMessage, AgentTriggerStatusRunning)
	if err != nil {
		return false, err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}
