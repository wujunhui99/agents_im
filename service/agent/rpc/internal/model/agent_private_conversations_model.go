package model

import (
	"context"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ AgentPrivateConversationsModel = (*customAgentPrivateConversationsModel)(nil)

const (
	// AgentPrivateConversationStateIdle：无在途 agent 回复，下条消息可立刻驱动一轮。
	AgentPrivateConversationStateIdle = "idle"
	// AgentPrivateConversationStateRunning：一轮回复在途，入站消息只累积到 pending（合流）。
	AgentPrivateConversationStateRunning = "running"
)

type (
	// AgentPrivateConversationsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customAgentPrivateConversationsModel.
	AgentPrivateConversationsModel interface {
		agentPrivateConversationsModel
		withSession(session sqlx.Session) AgentPrivateConversationsModel

		// AppendPending 把一条入站 user 消息追加到会话 pending，并按状态机决定是否由本次调用驱动
		// 一轮回复（fired）：
		//   · 会话不存在 → 建行、pending=[elem]、state=running，fired=true；
		//   · seq<=last_consumed_seq → 重复/回放，直接丢弃，fired=false（冪等）；
		//   · state=idle 或 running 但已过 running_until 租约（崩溃残留）→ 追加 + 抢占 running，fired=true；
		//   · state=running 且租约未过 → 仅追加 pending，fired=false（在途那轮末尾会 CommitRound 再消费）。
		// pendingElemJSON 是单个 pending 元素的 json 对象串；runningTTLMillis 是 running 租约。
		AppendPending(ctx context.Context, in AppendPendingInput) (bool, error)

		// CommitRound 收束一轮：把 roundJSON 追加进 rounds 并裁剪到最近 maxRounds 轮；从 pending 移除
		// 本轮已消费（seq<=consumedUpToSeq）的元素；若仍有更晚的 pending 残留则保持 running 并续租
		// （返回 morePending=true，驱动 run loop 再跑一轮），否则落 idle。整个收束在一事务内完成。
		CommitRound(ctx context.Context, in CommitRoundInput) (bool, error)

		// DiscardPending 与 CommitRound 同样清掉 seq<=consumedUpToSeq 的 pending 并推进 running/idle，
		// 但**不**往 rounds 追加轮次——用于一轮 run 失败时消费掉该批 user 消息（避免对同一失败批
		// 无限重试），失败提示已由 runner 直接发回用户，故不落 assistant 历史。返回 morePending。
		DiscardPending(ctx context.Context, conversationID string, consumedUpToSeq int64, runningTTLMillis int64) (bool, error)
	}

	// AppendPendingInput 见 AppendPending。
	AppendPendingInput struct {
		ConversationID   string
		AgentAccountID   string
		PeerAccountID    string
		PendingElemJSON  string
		Seq              int64
		RunningTTLMillis int64
	}

	// CommitRoundInput 见 CommitRound。
	CommitRoundInput struct {
		ConversationID   string
		RoundJSON        string
		ConsumedUpToSeq  int64
		MaxRounds        int64
		RunningTTLMillis int64
	}

	customAgentPrivateConversationsModel struct {
		*defaultAgentPrivateConversationsModel
	}
)

// NewAgentPrivateConversationsModel returns a model for the database table.
func NewAgentPrivateConversationsModel(conn sqlx.SqlConn) AgentPrivateConversationsModel {
	return &customAgentPrivateConversationsModel{
		defaultAgentPrivateConversationsModel: newAgentPrivateConversationsModel(conn),
	}
}

func (m *customAgentPrivateConversationsModel) withSession(session sqlx.Session) AgentPrivateConversationsModel {
	return NewAgentPrivateConversationsModel(sqlx.NewSqlConnFromSession(session))
}

func (m *customAgentPrivateConversationsModel) AppendPending(ctx context.Context, in AppendPendingInput) (bool, error) {
	ttl := max(in.RunningTTLMillis, 1)
	var fired bool
	err := m.conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		// 1) 会话不存在 → 建行并占用 running（本次驱动首轮）。
		result, err := session.ExecCtx(ctx, "insert into "+m.table+` (
  conversation_id, agent_account_id, peer_account_id, rounds, pending, last_consumed_seq, state, running_until, version
)
values ($1, $2, $3, '[]'::jsonb, jsonb_build_array($4::jsonb), $5, `+"'"+AgentPrivateConversationStateRunning+"'"+`, now() + ($6 * interval '1 millisecond'), 0)
on conflict (conversation_id) do nothing
`, in.ConversationID, in.AgentAccountID, in.PeerAccountID, in.PendingElemJSON, in.Seq, ttl)
		if err != nil {
			return err
		}
		if rows, err := result.RowsAffected(); err != nil {
			return err
		} else if rows > 0 {
			fired = true
			return nil
		}

		// 2) 既有行 + 新 seq + (idle 或 running 租约已过) → 追加并抢占 running（本次驱动一轮）。
		result, err = session.ExecCtx(ctx, "update "+m.table+`
set pending = pending || $2::jsonb,
    last_consumed_seq = $3,
    state = `+"'"+AgentPrivateConversationStateRunning+"'"+`,
    running_until = now() + ($4 * interval '1 millisecond'),
    version = version + 1,
    updated_at = now()
where conversation_id = $1
  and $3 > last_consumed_seq
  and (state = `+"'"+AgentPrivateConversationStateIdle+"'"+` or running_until is null or running_until <= now())
`, in.ConversationID, in.PendingElemJSON, in.Seq, ttl)
		if err != nil {
			return err
		}
		if rows, err := result.RowsAffected(); err != nil {
			return err
		} else if rows > 0 {
			fired = true
			return nil
		}

		// 3) 既有行 + 新 seq + running 租约未过 → 仅累积 pending（在途轮末尾 CommitRound 会消费），
		//    不驱动新一轮。seq<=last_consumed_seq 时本 UPDATE 亦 0 行（冪等丢弃）。
		_, err = session.ExecCtx(ctx, "update "+m.table+`
set pending = pending || $2::jsonb,
    last_consumed_seq = $3,
    version = version + 1,
    updated_at = now()
where conversation_id = $1 and $3 > last_consumed_seq
`, in.ConversationID, in.PendingElemJSON, in.Seq)
		if err != nil {
			return err
		}
		fired = false
		return nil
	})
	if err != nil {
		return false, err
	}
	return fired, nil
}

func (m *customAgentPrivateConversationsModel) CommitRound(ctx context.Context, in CommitRoundInput) (bool, error) {
	maxRounds := max(in.MaxRounds, 1)
	ttl := max(in.RunningTTLMillis, 1)
	var morePending bool
	// rounds 追加后按 ordinality 保留最后 maxRounds 轮；pending 过滤掉 seq<=consumedUpToSeq；
	// 残留 pending 决定 running/idle。SET 各 RHS 读的都是行的旧值（Postgres 语义），故 pending
	// 过滤与 state 判定都基于同一旧 pending，一致。
	err := m.conn.QueryRowCtx(ctx, &morePending, "update "+m.table+`
set rounds = coalesce((
      select jsonb_agg(elem order by ord)
      from jsonb_array_elements(rounds || $2::jsonb) with ordinality as x(elem, ord)
      where ord > greatest(0, jsonb_array_length(rounds || $2::jsonb) - $3)
    ), '[]'::jsonb),
    pending = coalesce((
      select jsonb_agg(elem order by ord)
      from jsonb_array_elements(pending) with ordinality as x(elem, ord)
      where (elem->>'seq')::bigint > $4
    ), '[]'::jsonb),
    state = case when exists (
              select 1 from jsonb_array_elements(pending) e where (e->>'seq')::bigint > $4
            ) then `+"'"+AgentPrivateConversationStateRunning+"'"+` else `+"'"+AgentPrivateConversationStateIdle+"'"+` end,
    running_until = case when exists (
              select 1 from jsonb_array_elements(pending) e where (e->>'seq')::bigint > $4
            ) then now() + ($5 * interval '1 millisecond') else null end,
    version = version + 1,
    updated_at = now()
where conversation_id = $1
returning state = `+"'"+AgentPrivateConversationStateRunning+"'"+`
`, in.ConversationID, in.RoundJSON, maxRounds, in.ConsumedUpToSeq, ttl)
	if err != nil {
		if err == sqlx.ErrNotFound {
			return false, ErrNotFound
		}
		return false, err
	}
	return morePending, nil
}

func (m *customAgentPrivateConversationsModel) DiscardPending(ctx context.Context, conversationID string, consumedUpToSeq int64, runningTTLMillis int64) (bool, error) {
	ttl := max(runningTTLMillis, 1)
	var morePending bool
	err := m.conn.QueryRowCtx(ctx, &morePending, "update "+m.table+`
set pending = coalesce((
      select jsonb_agg(elem order by ord)
      from jsonb_array_elements(pending) with ordinality as x(elem, ord)
      where (elem->>'seq')::bigint > $2
    ), '[]'::jsonb),
    state = case when exists (
              select 1 from jsonb_array_elements(pending) e where (e->>'seq')::bigint > $2
            ) then `+"'"+AgentPrivateConversationStateRunning+"'"+` else `+"'"+AgentPrivateConversationStateIdle+"'"+` end,
    running_until = case when exists (
              select 1 from jsonb_array_elements(pending) e where (e->>'seq')::bigint > $2
            ) then now() + ($3 * interval '1 millisecond') else null end,
    version = version + 1,
    updated_at = now()
where conversation_id = $1
returning state = `+"'"+AgentPrivateConversationStateRunning+"'"+`
`, conversationID, consumedUpToSeq, ttl)
	if err != nil {
		if err == sqlx.ErrNotFound {
			return false, ErrNotFound
		}
		return false, err
	}
	return morePending, nil
}
