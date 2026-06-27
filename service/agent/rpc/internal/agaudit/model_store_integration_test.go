//go:build integration

package agaudit_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/wujunhui99/agents_im/pkg/agentaudit"
	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/service/agent/rpc/internal/agaudit"
)

// TestPostgresAgentAuditStore 验证 agent 域审计四表的 goctl model store（#616）对齐旧
// internal/repository 语义：bigint keystone `::text` 读写、summary 脱敏、run-not-found 守卫、
// append-only 触发器拒绝 UPDATE。需已迁移 002/013 的 PG。
func TestPostgresAgentAuditStore(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = os.Getenv("AGENTS_IM_POSTGRES_DSN")
	}
	if dsn == "" {
		t.Skip("DATABASE_URL or AGENTS_IM_POSTGRES_DSN is required for agaudit integration tests")
	}

	ctx := context.Background()
	store := agaudit.NewModelStore(dsn)
	// bigint keystone：run_id/agent_id/tool_call_id 必须是数字串；用纳秒派生唯一 ID。
	base := time.Now().UnixNano() % 1_000_000_000
	runID := fmt.Sprintf("%d", 900_000_000_000+base)
	agentID := fmt.Sprintf("%d", 800_000_000_000+base)
	toolCallID := fmt.Sprintf("%d", 700_000_000_000+base)

	run, err := store.CreateAgentRun(ctx, agentaudit.CreateRunInput{
		RunID:          runID,
		AgentID:        agentID,
		ConversationID: "single:usr_pg_1:agent_pg_1",
		Status:         agentaudit.StatusSucceeded,
		InputSummary: agentaudit.Summary{
			"prompt":       "hello",
			"access_token": "must-not-leak",
		},
		TraceID:   fmt.Sprintf("trace_pg_%d", base),
		RequestID: fmt.Sprintf("req_pg_%d", base),
	})
	if err != nil {
		t.Fatalf("create agent run: %v", err)
	}
	if run.InputSummary["access_token"] != agentaudit.RedactedValue {
		t.Fatalf("run summary did not redact token: %+v", run.InputSummary)
	}

	// 子表写入要求 run 存在；不存在的 run → NotFound。
	if _, err := store.ListAgentToolCallsByRunID(ctx, "111111111111"); apperror.From(err).Code != apperror.CodeNotFound {
		t.Fatalf("tool calls for missing run = %v, want not found", err)
	}

	if _, err := store.CreateAgentToolCall(ctx, agentaudit.CreateToolCallInput{
		ToolCallID: toolCallID,
		RunID:      run.RunID,
		AgentID:    run.AgentID,
		ToolName:   "im.get_conversation_context",
		Status:     agentaudit.StatusSucceeded,
	}); err != nil {
		t.Fatalf("create tool call: %v", err)
	}
	toolCalls, err := store.ListAgentToolCallsByRunID(ctx, run.RunID)
	if err != nil {
		t.Fatalf("list tool calls: %v", err)
	}
	if len(toolCalls) != 1 || toolCalls[0].ToolCallID != toolCallID {
		t.Fatalf("tool calls mismatch: %+v", toolCalls)
	}

	// trace_id 命中最新 run。
	if got, err := store.GetAgentRunByTraceID(ctx, run.TraceID); err != nil || got.RunID != run.RunID {
		t.Fatalf("get by trace = %+v err=%v", got, err)
	}

	// append-only：触发器拒绝 UPDATE。
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open pg: %v", err)
	}
	defer db.Close()
	if _, err := db.ExecContext(ctx, `update agent_runs set status = 'failed' where run_id = $1::bigint`, run.RunID); err == nil {
		t.Fatal("expected append-only trigger to reject update")
	}
}
