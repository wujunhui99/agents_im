package model

import (
	"context"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// registry_writes.go 给注册表 goctl model 补 #606 写路径所需的自定义方法（returning-insert、
// upsert-by-name、active 列表、绑定的幂等 bind / 原子 replace）。jsonb 列写入显式 ::jsonb 转换；
// bigint 自增 id 由库生成。绑定的 ReplaceForAgent 是单表原子操作（delete+insert 同事务），
// 属 model 级原语，不编排跨表业务事务（业务事务边界仍在 Logic 层）。

const (
	agentPromptsReturning = "prompt_id, name, description, content, variables_schema_json, version, status, created_by, created_at, updated_at"
	agentToolsReturning   = "tool_id, name, description, tool_type, mcp_server_id, mcp_tool_name, local_handler_key, builtin_key, input_schema_json, output_schema_json, permission_level, status, admin_configured, created_by, created_at, updated_at"
	agentSkillsReturning  = "skill_id, name, description, version, object_key, sha256, content_type, size_bytes, status, created_by, created_at, updated_at"
	mcpServersReturning   = "server_id, name, transport, url, config_json, headers_secret_ref, timeout_seconds, status, admin_configured, created_by, created_at, updated_at"

	agentPromptBindingsReturning = "agent_id, prompt_id, created_by, created_at, updated_at, id"
	agentToolBindingsReturning   = "agent_id, tool_id, created_by, created_at, updated_at, id"
	agentSkillBindingsReturning  = "agent_id, skill_id, created_by, created_at, updated_at, id"
)

// ---- agent_prompts ----

func (m *customAgentPromptsModel) InsertReturning(ctx context.Context, data *AgentPrompts) (*AgentPrompts, error) {
	query := fmt.Sprintf(`insert into %s (name, description, content, variables_schema_json, version, status, created_by)
values ($1, $2, $3, $4::jsonb, $5, $6, $7)
returning %s`, m.table, agentPromptsReturning)
	var resp AgentPrompts
	if err := m.conn.QueryRowCtx(ctx, &resp, query, data.Name, data.Description, data.Content, data.VariablesSchemaJson, data.Version, data.Status, data.CreatedBy); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ---- mcp_servers ----

func (m *customMcpServersModel) InsertReturning(ctx context.Context, data *McpServers) (*McpServers, error) {
	query := fmt.Sprintf(`insert into %s (name, transport, url, config_json, headers_secret_ref, timeout_seconds, status, admin_configured, created_by)
values ($1, $2, $3, $4::jsonb, $5, $6, $7, $8, $9)
returning %s`, m.table, mcpServersReturning)
	var resp McpServers
	if err := m.conn.QueryRowCtx(ctx, &resp, query, data.Name, data.Transport, data.Url, data.ConfigJson, data.HeadersSecretRef, data.TimeoutSeconds, data.Status, data.AdminConfigured, data.CreatedBy); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ---- agent_tools ----

func (m *customAgentToolsModel) InsertReturning(ctx context.Context, data *AgentTools) (*AgentTools, error) {
	query := fmt.Sprintf(`insert into %s (name, description, tool_type, mcp_server_id, mcp_tool_name, local_handler_key, builtin_key, input_schema_json, output_schema_json, permission_level, status, admin_configured, created_by)
values ($1, $2, $3, $4, $5, $6, $7, $8::jsonb, $9::jsonb, $10, $11, $12, $13)
returning %s`, m.table, agentToolsReturning)
	var resp AgentTools
	if err := m.conn.QueryRowCtx(ctx, &resp, query, data.Name, data.Description, data.ToolType, data.McpServerId, data.McpToolName, data.LocalHandlerKey, data.BuiltinKey, data.InputSchemaJson, data.OutputSchemaJson, data.PermissionLevel, data.Status, data.AdminConfigured, data.CreatedBy); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (m *customAgentToolsModel) UpsertByName(ctx context.Context, data *AgentTools) (*AgentTools, error) {
	query := fmt.Sprintf(`insert into %s (name, description, tool_type, mcp_server_id, mcp_tool_name, local_handler_key, builtin_key, input_schema_json, output_schema_json, permission_level, status, admin_configured, created_by)
values ($1, $2, $3, $4, $5, $6, $7, $8::jsonb, $9::jsonb, $10, $11, $12, $13)
on conflict (name) do update
set description = excluded.description,
    tool_type = excluded.tool_type,
    mcp_server_id = excluded.mcp_server_id,
    mcp_tool_name = excluded.mcp_tool_name,
    local_handler_key = excluded.local_handler_key,
    builtin_key = excluded.builtin_key,
    input_schema_json = excluded.input_schema_json,
    output_schema_json = excluded.output_schema_json,
    permission_level = excluded.permission_level,
    status = excluded.status,
    admin_configured = excluded.admin_configured,
    created_by = excluded.created_by,
    updated_at = now()
returning %s`, m.table, agentToolsReturning)
	var resp AgentTools
	if err := m.conn.QueryRowCtx(ctx, &resp, query, data.Name, data.Description, data.ToolType, data.McpServerId, data.McpToolName, data.LocalHandlerKey, data.BuiltinKey, data.InputSchemaJson, data.OutputSchemaJson, data.PermissionLevel, data.Status, data.AdminConfigured, data.CreatedBy); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (m *customAgentToolsModel) ListActive(ctx context.Context) ([]*AgentTools, error) {
	query := fmt.Sprintf("select %s from %s where status = 'active' order by name", agentToolsReturning, m.table)
	var rows []*AgentTools
	if err := m.conn.QueryRowsCtx(ctx, &rows, query); err != nil {
		return nil, err
	}
	return rows, nil
}

// ---- agent_skills ----

func (m *customAgentSkillsModel) InsertReturning(ctx context.Context, data *AgentSkills) (*AgentSkills, error) {
	query := fmt.Sprintf(`insert into %s (name, description, version, object_key, sha256, content_type, size_bytes, status, created_by)
values ($1, $2, $3, $4, $5, $6, $7, $8, $9)
returning %s`, m.table, agentSkillsReturning)
	var resp AgentSkills
	if err := m.conn.QueryRowCtx(ctx, &resp, query, data.Name, data.Description, data.Version, data.ObjectKey, data.Sha256, data.ContentType, data.SizeBytes, data.Status, data.CreatedBy); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ---- agent_prompt_bindings ----

func (m *customAgentPromptBindingsModel) BindOne(ctx context.Context, agentID, promptID int64, createdBy string) (*AgentPromptBindings, bool, error) {
	existing, err := m.FindOneByAgentIdPromptId(ctx, agentID, promptID)
	if err == nil {
		return existing, false, nil
	}
	if err != ErrNotFound {
		return nil, false, err
	}
	row, err := m.insertReturning(ctx, m.conn, agentID, promptID, createdBy)
	if err != nil {
		return nil, false, err
	}
	return row, true, nil
}

func (m *customAgentPromptBindingsModel) ReplaceForAgent(ctx context.Context, agentID int64, promptIDs []int64, createdBy string) ([]*AgentPromptBindings, error) {
	rows := make([]*AgentPromptBindings, 0, len(promptIDs))
	err := m.conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		if _, err := session.ExecCtx(ctx, fmt.Sprintf("delete from %s where agent_id = $1", m.table), agentID); err != nil {
			return err
		}
		seen := make(map[int64]struct{}, len(promptIDs))
		for _, promptID := range promptIDs {
			if _, ok := seen[promptID]; ok {
				continue
			}
			seen[promptID] = struct{}{}
			row, err := m.insertReturning(ctx, session, agentID, promptID, createdBy)
			if err != nil {
				return err
			}
			rows = append(rows, row)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func (m *customAgentPromptBindingsModel) insertReturning(ctx context.Context, session sqlx.Session, agentID, promptID int64, createdBy string) (*AgentPromptBindings, error) {
	query := fmt.Sprintf(`insert into %s (agent_id, prompt_id, created_by) values ($1, $2, $3) returning %s`, m.table, agentPromptBindingsReturning)
	var resp AgentPromptBindings
	if err := session.QueryRowCtx(ctx, &resp, query, agentID, promptID, createdBy); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ---- agent_tool_bindings ----

func (m *customAgentToolBindingsModel) BindOne(ctx context.Context, agentID, toolID int64, createdBy string) (*AgentToolBindings, bool, error) {
	existing, err := m.FindOneByAgentIdToolId(ctx, agentID, toolID)
	if err == nil {
		return existing, false, nil
	}
	if err != ErrNotFound {
		return nil, false, err
	}
	row, err := m.insertReturning(ctx, m.conn, agentID, toolID, createdBy)
	if err != nil {
		return nil, false, err
	}
	return row, true, nil
}

func (m *customAgentToolBindingsModel) ReplaceForAgent(ctx context.Context, agentID int64, toolIDs []int64, createdBy string) ([]*AgentToolBindings, error) {
	rows := make([]*AgentToolBindings, 0, len(toolIDs))
	err := m.conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		if _, err := session.ExecCtx(ctx, fmt.Sprintf("delete from %s where agent_id = $1", m.table), agentID); err != nil {
			return err
		}
		seen := make(map[int64]struct{}, len(toolIDs))
		for _, toolID := range toolIDs {
			if _, ok := seen[toolID]; ok {
				continue
			}
			seen[toolID] = struct{}{}
			row, err := m.insertReturning(ctx, session, agentID, toolID, createdBy)
			if err != nil {
				return err
			}
			rows = append(rows, row)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func (m *customAgentToolBindingsModel) insertReturning(ctx context.Context, session sqlx.Session, agentID, toolID int64, createdBy string) (*AgentToolBindings, error) {
	query := fmt.Sprintf(`insert into %s (agent_id, tool_id, created_by) values ($1, $2, $3) returning %s`, m.table, agentToolBindingsReturning)
	var resp AgentToolBindings
	if err := session.QueryRowCtx(ctx, &resp, query, agentID, toolID, createdBy); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ---- agent_skill_bindings ----

func (m *customAgentSkillBindingsModel) BindOne(ctx context.Context, agentID, skillID int64, createdBy string) (*AgentSkillBindings, bool, error) {
	existing, err := m.FindOneByAgentIdSkillId(ctx, agentID, skillID)
	if err == nil {
		return existing, false, nil
	}
	if err != ErrNotFound {
		return nil, false, err
	}
	query := fmt.Sprintf(`insert into %s (agent_id, skill_id, created_by) values ($1, $2, $3) returning %s`, m.table, agentSkillBindingsReturning)
	var resp AgentSkillBindings
	if err := m.conn.QueryRowCtx(ctx, &resp, query, agentID, skillID, createdBy); err != nil {
		return nil, false, err
	}
	return &resp, true, nil
}
