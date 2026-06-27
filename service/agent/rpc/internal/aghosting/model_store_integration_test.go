//go:build integration

package aghosting_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/service/agent/rpc/internal/aghosting"
)

// TestPostgresAgentHostingStore 验证 agent 域 agent_conversation_hosting +
// agent_trigger_idempotency 的 goctl model store（#670）对齐旧 internal/repository 语义：
// conversation_id 主键 upsert、TTL 抢占（failed 总可抢、running 仅超 TTL 可抢、succeeded 不可抢）、
// Finish 终态推进幂等。需已迁移 003 的 PG。
func TestPostgresAgentHostingStore(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = os.Getenv("AGENTS_IM_POSTGRES_DSN")
	}
	if dsn == "" {
		t.Skip("DATABASE_URL or AGENTS_IM_POSTGRES_DSN is required for aghosting integration tests")
	}

	ctx := context.Background()
	store := aghosting.NewModelStore(dsn)
	uniq := time.Now().UnixNano()
	conversationID := fmt.Sprintf("single:usr_ah_%d:usr_ah_agent_%d", uniq, uniq)
	agentAccountID := fmt.Sprintf("usr_ah_agent_%d", uniq)

	// 缺失行 → NotFound。
	if _, err := store.GetAgentConversationHosting(ctx, conversationID); apperror.From(err).Code != apperror.CodeNotFound {
		t.Fatalf("missing hosting error = %v, want not found", err)
	}

	saved, err := store.UpsertAgentConversationHosting(ctx, aghosting.AgentConversationHosting{
		ConversationID:             conversationID,
		AgentAccountID:             agentAccountID,
		Enabled:                    true,
		AllowAgentMessageRecursion: true,
	})
	if err != nil {
		t.Fatalf("upsert hosting: %v", err)
	}
	if !saved.Enabled || saved.AgentAccountID != agentAccountID || !saved.AllowAgentMessageRecursion {
		t.Fatalf("saved hosting mismatch: %+v", saved)
	}
	if got, err := store.GetAgentConversationHosting(ctx, conversationID); err != nil || got.AgentAccountID != agentAccountID {
		t.Fatalf("get hosting = %+v err=%v", got, err)
	}

	key := fmt.Sprintf("evt_ah_%d:%s", uniq, agentAccountID)
	startInput := aghosting.AgentTriggerStartInput{
		IdempotencyKey:     key,
		ConversationID:     conversationID,
		AgentAccountID:     agentAccountID,
		TriggerServerMsgID: "msg_trigger",
		TriggerEventID:     "evt_trigger",
		RunningTTL:         time.Hour,
	}

	// 首次占用成功；同 key 重复（running 未超 TTL）失败。
	if started, err := store.TryStartAgentTrigger(ctx, startInput); err != nil || !started {
		t.Fatalf("first start = (%v, %v), want (true, nil)", started, err)
	}
	if started, err := store.TryStartAgentTrigger(ctx, startInput); err != nil || started {
		t.Fatalf("fresh running re-start = (%v, %v), want (false, nil)", started, err)
	}

	// 终态推进：running → failed；failed 可被重新抢占。
	if err := store.FinishAgentTrigger(ctx, aghosting.AgentTriggerFinishInput{
		IdempotencyKey: key,
		Status:         aghosting.AgentTriggerStatusFailed,
		ErrorMessage:   "runtime failed",
	}); err != nil {
		t.Fatalf("finish failed: %v", err)
	}
	// 重复 Finish 终态 → NotFound（status != running）。
	if err := store.FinishAgentTrigger(ctx, aghosting.AgentTriggerFinishInput{
		IdempotencyKey: key,
		Status:         aghosting.AgentTriggerStatusFailed,
		ErrorMessage:   "again",
	}); apperror.From(err).Code != apperror.CodeNotFound {
		t.Fatalf("re-finish terminal = %v, want not found", err)
	}
	if started, err := store.TryStartAgentTrigger(ctx, startInput); err != nil || !started {
		t.Fatalf("failed re-start = (%v, %v), want (true, nil)", started, err)
	}

	// succeeded 后不可再抢占。
	if err := store.FinishAgentTrigger(ctx, aghosting.AgentTriggerFinishInput{
		IdempotencyKey:      key,
		Status:              aghosting.AgentTriggerStatusSucceeded,
		ResponseServerMsgID: "msg_response",
	}); err != nil {
		t.Fatalf("finish succeeded: %v", err)
	}
	if started, err := store.TryStartAgentTrigger(ctx, startInput); err != nil || started {
		t.Fatalf("succeeded re-start = (%v, %v), want (false, nil)", started, err)
	}
}
