//go:build integration

package registry_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/zeromicro/go-zero/core/stores/postgres"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/pkg/model"
	"github.com/wujunhui99/agents_im/service/agent/rpc/internal/registry"
)

// TestPostgresRegistryStore 验证 agent 域注册表 goctl model 只读 Store(#605)对齐旧
// internal/repository 语义:bigint 列读出为 string ID、NullInt64 mcp_server_id 处理、
// 绑定按 agent 列表查询、缺失行翻译 NotFound。需已迁移到 025 的 PG。
func TestPostgresRegistryStore(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = os.Getenv("AGENTS_IM_POSTGRES_DSN")
	}
	if dsn == "" {
		t.Skip("DATABASE_URL or AGENTS_IM_POSTGRES_DSN is required for registry integration tests")
	}

	ctx := context.Background()
	conn := postgres.New(dsn)
	store := registry.NewStoreFromConn(conn)

	uniq := time.Now().UnixNano()
	createdBy := fmt.Sprintf("usr_reg_%d", uniq)
	agentID := uniq // bigint agent_id (注册表绑定列已迁 bigint)

	// 缺失行 → NotFound。
	if _, err := store.GetPrompt(ctx, "999999999999"); apperror.From(err).Code != apperror.CodeNotFound {
		t.Fatalf("missing prompt error = %v, want not found", err)
	}
	// 非数字 ID → NotFound(string↔int64 边界)。
	if _, err := store.GetTool(ctx, "not-a-number"); apperror.From(err).Code != apperror.CodeNotFound {
		t.Fatalf("non-numeric tool id error = %v, want not found", err)
	}

	// seed prompt + binding。
	var promptID int64
	if err := conn.QueryRowCtx(ctx, &promptID, `
insert into agent_prompts (name, description, content, variables_schema_json, version, status, created_by)
values ($1, '', 'hi', '{}'::jsonb, $2, 'active', $3) returning prompt_id`,
		fmt.Sprintf("p_%d", uniq), fmt.Sprintf("v%d", uniq), createdBy); err != nil {
		t.Fatalf("seed prompt: %v", err)
	}
	if _, err := conn.ExecCtx(ctx, `insert into agent_prompt_bindings (agent_id, prompt_id, created_by) values ($1, $2, $3)`,
		agentID, promptID, createdBy); err != nil {
		t.Fatalf("seed prompt binding: %v", err)
	}

	gotPrompt, err := store.GetPrompt(ctx, fmt.Sprintf("%d", promptID))
	if err != nil {
		t.Fatalf("GetPrompt: %v", err)
	}
	if gotPrompt.PromptID != fmt.Sprintf("%d", promptID) || gotPrompt.Status != model.AgentPromptStatusActive {
		t.Fatalf("GetPrompt = %+v, want string id %d active", gotPrompt, promptID)
	}
	promptBindings, err := store.ListPromptBindings(ctx, fmt.Sprintf("%d", agentID))
	if err != nil {
		t.Fatalf("ListPromptBindings: %v", err)
	}
	if len(promptBindings) != 1 || promptBindings[0].PromptID != fmt.Sprintf("%d", promptID) {
		t.Fatalf("ListPromptBindings = %+v, want one binding to %d", promptBindings, promptID)
	}

	// seed mcp server + mcp tool,验证 NullInt64 mcp_server_id 读出为 string。
	var serverID int64
	if err := conn.QueryRowCtx(ctx, &serverID, `
insert into mcp_servers (name, transport, url, config_json, headers_secret_ref, timeout_seconds, status, admin_configured, created_by)
values ($1, 'http', 'https://example.com', '{}'::jsonb, '', 30, 'active', true, $2) returning server_id`,
		fmt.Sprintf("mcp_%d", uniq), createdBy); err != nil {
		t.Fatalf("seed mcp server: %v", err)
	}
	var toolID int64
	if err := conn.QueryRowCtx(ctx, &toolID, `
insert into agent_tools (name, description, tool_type, mcp_server_id, mcp_tool_name, input_schema_json, output_schema_json, permission_level, status, admin_configured, created_by)
values ($1, '', 'mcp', $2, 'remote_tool', '{}'::jsonb, '{}'::jsonb, 'agent_bound', 'active', true, $3) returning tool_id`,
		fmt.Sprintf("t_%d", uniq), serverID, createdBy); err != nil {
		t.Fatalf("seed tool: %v", err)
	}
	if _, err := conn.ExecCtx(ctx, `insert into agent_tool_bindings (agent_id, tool_id, created_by) values ($1, $2, $3)`,
		agentID, toolID, createdBy); err != nil {
		t.Fatalf("seed tool binding: %v", err)
	}

	gotTool, err := store.GetTool(ctx, fmt.Sprintf("%d", toolID))
	if err != nil {
		t.Fatalf("GetTool: %v", err)
	}
	if gotTool.MCPServerID != fmt.Sprintf("%d", serverID) {
		t.Fatalf("GetTool.MCPServerID = %q, want %d", gotTool.MCPServerID, serverID)
	}
	server, err := store.GetMCPServer(ctx, fmt.Sprintf("%d", serverID))
	if err != nil {
		t.Fatalf("GetMCPServer: %v", err)
	}
	if server.ServerID != fmt.Sprintf("%d", serverID) || server.TimeoutSeconds != 30 {
		t.Fatalf("GetMCPServer = %+v, want string id %d timeout 30", server, serverID)
	}
	binding, err := store.GetToolBinding(ctx, fmt.Sprintf("%d", agentID), fmt.Sprintf("%d", toolID))
	if err != nil {
		t.Fatalf("GetToolBinding: %v", err)
	}
	if binding.ToolID != fmt.Sprintf("%d", toolID) {
		t.Fatalf("GetToolBinding = %+v, want tool %d", binding, toolID)
	}
	toolBindings, err := store.ListToolBindings(ctx, fmt.Sprintf("%d", agentID))
	if err != nil {
		t.Fatalf("ListToolBindings: %v", err)
	}
	if len(toolBindings) != 1 || toolBindings[0].ToolID != fmt.Sprintf("%d", toolID) {
		t.Fatalf("ListToolBindings = %+v, want one binding to %d", toolBindings, toolID)
	}
}
