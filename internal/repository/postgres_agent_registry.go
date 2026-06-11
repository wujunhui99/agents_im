package repository

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/wujunhui99/agents_im/common/share/model"
	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/zeromicro/go-zero/core/stores/postgres"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type postgresAgentPromptRow struct {
	PromptID            string    `db:"prompt_id"`
	Name                string    `db:"name"`
	Description         string    `db:"description"`
	Content             string    `db:"content"`
	VariablesSchemaJSON []byte    `db:"variables_schema_json"`
	Version             string    `db:"version"`
	Status              string    `db:"status"`
	CreatedBy           string    `db:"created_by"`
	CreatedAt           time.Time `db:"created_at"`
	UpdatedAt           time.Time `db:"updated_at"`
}

type postgresAgentMCPServerRow struct {
	ServerID         string    `db:"server_id"`
	Name             string    `db:"name"`
	Transport        string    `db:"transport"`
	URL              string    `db:"url"`
	ConfigJSON       []byte    `db:"config_json"`
	HeadersSecretRef string    `db:"headers_secret_ref"`
	TimeoutSeconds   int       `db:"timeout_seconds"`
	Status           string    `db:"status"`
	AdminConfigured  bool      `db:"admin_configured"`
	CreatedBy        string    `db:"created_by"`
	CreatedAt        time.Time `db:"created_at"`
	UpdatedAt        time.Time `db:"updated_at"`
}

type postgresAgentToolRow struct {
	ToolID           string         `db:"tool_id"`
	Name             string         `db:"name"`
	Description      string         `db:"description"`
	ToolType         string         `db:"tool_type"`
	MCPServerID      sql.NullString `db:"mcp_server_id"`
	MCPToolName      string         `db:"mcp_tool_name"`
	LocalHandlerKey  string         `db:"local_handler_key"`
	BuiltinKey       string         `db:"builtin_key"`
	InputSchemaJSON  []byte         `db:"input_schema_json"`
	OutputSchemaJSON []byte         `db:"output_schema_json"`
	PermissionLevel  string         `db:"permission_level"`
	Status           string         `db:"status"`
	AdminConfigured  bool           `db:"admin_configured"`
	CreatedBy        string         `db:"created_by"`
	CreatedAt        time.Time      `db:"created_at"`
	UpdatedAt        time.Time      `db:"updated_at"`
}

type postgresAgentSkillRow struct {
	SkillID     string    `db:"skill_id"`
	Name        string    `db:"name"`
	Description string    `db:"description"`
	Version     string    `db:"version"`
	ObjectKey   string    `db:"object_key"`
	SHA256      string    `db:"sha256"`
	ContentType string    `db:"content_type"`
	SizeBytes   int64     `db:"size_bytes"`
	Status      string    `db:"status"`
	CreatedBy   string    `db:"created_by"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}

type postgresAgentPromptBindingRow struct {
	AgentID   string    `db:"agent_id"`
	PromptID  string    `db:"prompt_id"`
	CreatedBy string    `db:"created_by"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

type postgresAgentToolBindingRow struct {
	AgentID   string    `db:"agent_id"`
	ToolID    string    `db:"tool_id"`
	CreatedBy string    `db:"created_by"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

type postgresAgentSkillBindingRow struct {
	AgentID   string    `db:"agent_id"`
	SkillID   string    `db:"skill_id"`
	CreatedBy string    `db:"created_by"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

func NewPostgresAgentRegistryRepository(dataSource string) (*PostgresRepository, error) {
	dataSource = strings.TrimSpace(dataSource)
	if dataSource == "" {
		return nil, sql.ErrConnDone
	}
	return NewPostgresRepositoryFromConn(postgres.New(dataSource)), nil
}

func (r *PostgresRepository) CreatePrompt(ctx context.Context, prompt model.AgentPrompt) (model.AgentPrompt, error) {
	var row postgresAgentPromptRow
	var err error
	if strings.TrimSpace(prompt.PromptID) == "" {
		err = r.conn.QueryRowCtx(ctx, &row, `
insert into agent_prompts (
  name, description, content, variables_schema_json, version, status, created_by
)
values ($1, $2, $3, $4::jsonb, $5, $6, $7)
returning prompt_id::text as prompt_id, name, description, content, variables_schema_json, version, status, created_by, created_at, updated_at
`, prompt.Name, prompt.Description, prompt.Content, prompt.VariablesSchemaJSON, prompt.Version, prompt.Status, prompt.CreatedBy)
	} else {
		err = r.conn.QueryRowCtx(ctx, &row, `
insert into agent_prompts (
  prompt_id, name, description, content, variables_schema_json, version, status, created_by
)
values ($1::bigint, $2, $3, $4, $5::jsonb, $6, $7, $8)
returning prompt_id::text as prompt_id, name, description, content, variables_schema_json, version, status, created_by, created_at, updated_at
`, prompt.PromptID, prompt.Name, prompt.Description, prompt.Content, prompt.VariablesSchemaJSON, prompt.Version, prompt.Status, prompt.CreatedBy)
	}
	if err != nil {
		return model.AgentPrompt{}, mapAgentRegistryPostgresWriteError(err, "prompt already exists", "invalid prompt")
	}
	return row.prompt(), nil
}

func (r *PostgresRepository) GetPrompt(ctx context.Context, promptID string) (model.AgentPrompt, error) {
	var row postgresAgentPromptRow
	err := r.conn.QueryRowCtx(ctx, &row, `
select prompt_id::text as prompt_id, name, description, content, variables_schema_json, version, status, created_by, created_at, updated_at
from agent_prompts
where prompt_id = $1::bigint
`, promptID)
	if err != nil {
		if isNotFound(err) {
			return model.AgentPrompt{}, apperror.NotFound("prompt not found")
		}
		return model.AgentPrompt{}, err
	}
	return row.prompt(), nil
}

func (r *PostgresRepository) GetPromptByNameVersion(ctx context.Context, name string, version string) (model.AgentPrompt, error) {
	var row postgresAgentPromptRow
	err := r.conn.QueryRowCtx(ctx, &row, `
select prompt_id::text as prompt_id, name, description, content, variables_schema_json, version, status, created_by, created_at, updated_at
from agent_prompts
where name = $1 and version = $2
`, name, version)
	if err != nil {
		if isNotFound(err) {
			return model.AgentPrompt{}, apperror.NotFound("prompt not found")
		}
		return model.AgentPrompt{}, err
	}
	return row.prompt(), nil
}

func (r *PostgresRepository) BindPrompt(ctx context.Context, binding model.AgentPromptBinding) (model.AgentPromptBinding, bool, error) {
	existing, err := queryAgentPromptBinding(ctx, r.conn, binding.AgentID, binding.PromptID)
	if err == nil {
		return existing.Clone(), false, nil
	}
	if err != nil && !isAgentRegistryNotFound(err) {
		return model.AgentPromptBinding{}, false, err
	}

	row, err := insertAgentPromptBinding(ctx, r.conn, binding)
	if err != nil {
		if isPostgresUniqueViolation(err) {
			existing, queryErr := queryAgentPromptBinding(ctx, r.conn, binding.AgentID, binding.PromptID)
			return existing.Clone(), false, queryErr
		}
		if isPostgresForeignKeyViolation(err) {
			return model.AgentPromptBinding{}, false, apperror.NotFound("prompt not found")
		}
		return model.AgentPromptBinding{}, false, err
	}
	return row.promptBinding(), true, nil
}

func (r *PostgresRepository) ListPromptBindings(ctx context.Context, agentID string) ([]model.AgentPromptBinding, error) {
	var rows []postgresAgentPromptBindingRow
	err := r.conn.QueryRowsCtx(ctx, &rows, `
select agent_id::text as agent_id, prompt_id::text as prompt_id, created_by, created_at, updated_at
from agent_prompt_bindings
where agent_id = $1::bigint
order by created_at desc, prompt_id
`, agentID)
	if err != nil {
		return nil, err
	}
	bindings := make([]model.AgentPromptBinding, 0, len(rows))
	for _, row := range rows {
		bindings = append(bindings, row.promptBinding())
	}
	return bindings, nil
}

func (r *PostgresRepository) ReplacePromptBindings(ctx context.Context, agentID string, promptIDs []string, createdBy string) ([]model.AgentPromptBinding, error) {
	bindings := make([]model.AgentPromptBinding, 0, len(promptIDs))
	err := r.withTx(ctx, func(ctx context.Context, session sqlx.Session) error {
		if _, err := session.ExecCtx(ctx, `
delete from agent_prompt_bindings
where agent_id = $1::bigint
`, agentID); err != nil {
			return err
		}
		seen := make(map[string]struct{}, len(promptIDs))
		for _, promptID := range promptIDs {
			if _, ok := seen[promptID]; ok {
				continue
			}
			seen[promptID] = struct{}{}
			row, err := insertAgentPromptBinding(ctx, session, model.AgentPromptBinding{
				AgentID:   agentID,
				PromptID:  promptID,
				CreatedBy: createdBy,
			})
			if err != nil {
				return err
			}
			bindings = append(bindings, row.promptBinding())
		}
		return nil
	})
	if err != nil {
		if isPostgresForeignKeyViolation(err) {
			return nil, apperror.NotFound("prompt not found")
		}
		if isPostgresCheckViolation(err) {
			return nil, apperror.InvalidArgument("invalid prompt binding")
		}
		return nil, err
	}
	return bindings, nil
}

func (r *PostgresRepository) CreateMCPServer(ctx context.Context, server model.AgentMCPServer) (model.AgentMCPServer, error) {
	var row postgresAgentMCPServerRow
	var err error
	if strings.TrimSpace(server.ServerID) == "" {
		err = r.conn.QueryRowCtx(ctx, &row, `
insert into mcp_servers (
  name, transport, url, config_json, headers_secret_ref, timeout_seconds, status, admin_configured, created_by
)
values ($1, $2, $3, $4::jsonb, $5, $6, $7, $8, $9)
returning server_id::text as server_id, name, transport, url, config_json, headers_secret_ref, timeout_seconds, status,
          admin_configured, created_by, created_at, updated_at
`, server.Name, server.Transport, server.URL, server.ConfigJSON, server.HeadersSecretRef, server.TimeoutSeconds, server.Status, server.AdminConfigured, server.CreatedBy)
	} else {
		err = r.conn.QueryRowCtx(ctx, &row, `
insert into mcp_servers (
  server_id, name, transport, url, config_json, headers_secret_ref, timeout_seconds, status, admin_configured, created_by
)
values ($1::bigint, $2, $3, $4, $5::jsonb, $6, $7, $8, $9, $10)
returning server_id::text as server_id, name, transport, url, config_json, headers_secret_ref, timeout_seconds, status,
          admin_configured, created_by, created_at, updated_at
`, server.ServerID, server.Name, server.Transport, server.URL, server.ConfigJSON, server.HeadersSecretRef, server.TimeoutSeconds, server.Status, server.AdminConfigured, server.CreatedBy)
	}
	if err != nil {
		return model.AgentMCPServer{}, mapAgentRegistryPostgresWriteError(err, "mcp server already exists", "invalid mcp server")
	}
	return row.mcpServer(), nil
}

func (r *PostgresRepository) GetMCPServer(ctx context.Context, serverID string) (model.AgentMCPServer, error) {
	var row postgresAgentMCPServerRow
	err := r.conn.QueryRowCtx(ctx, &row, `
select server_id::text as server_id, name, transport, url, config_json, headers_secret_ref, timeout_seconds, status,
       admin_configured, created_by, created_at, updated_at
from mcp_servers
where server_id = $1::bigint
`, serverID)
	if err != nil {
		if isNotFound(err) {
			return model.AgentMCPServer{}, apperror.NotFound("mcp server not found")
		}
		return model.AgentMCPServer{}, err
	}
	return row.mcpServer(), nil
}

func (r *PostgresRepository) RegisterTool(ctx context.Context, tool model.AgentTool) (model.AgentTool, error) {
	mcpServerID := nullableString(tool.MCPServerID)
	var row postgresAgentToolRow
	var err error
	if strings.TrimSpace(tool.ToolID) == "" {
		err = r.conn.QueryRowCtx(ctx, &row, `
insert into agent_tools (
  name, description, tool_type, mcp_server_id, mcp_tool_name, local_handler_key, builtin_key,
  input_schema_json, output_schema_json, permission_level, status, admin_configured, created_by
)
values ($1, $2, $3, $4::bigint, $5, $6, $7, $8::jsonb, $9::jsonb, $10, $11, $12, $13)
returning tool_id::text as tool_id, name, description, tool_type, mcp_server_id::text as mcp_server_id, mcp_tool_name, local_handler_key, builtin_key,
          input_schema_json, output_schema_json, permission_level, status, admin_configured, created_by, created_at, updated_at
`, tool.Name, tool.Description, tool.ToolType, mcpServerID, tool.MCPToolName, tool.LocalHandlerKey, tool.BuiltinKey,
			tool.InputSchemaJSON, tool.OutputSchemaJSON, tool.PermissionLevel, tool.Status, tool.AdminConfigured, tool.CreatedBy)
	} else {
		err = r.conn.QueryRowCtx(ctx, &row, `
insert into agent_tools (
  tool_id, name, description, tool_type, mcp_server_id, mcp_tool_name, local_handler_key, builtin_key,
  input_schema_json, output_schema_json, permission_level, status, admin_configured, created_by
)
values ($1::bigint, $2, $3, $4, $5::bigint, $6, $7, $8, $9::jsonb, $10::jsonb, $11, $12, $13, $14)
returning tool_id::text as tool_id, name, description, tool_type, mcp_server_id::text as mcp_server_id, mcp_tool_name, local_handler_key, builtin_key,
          input_schema_json, output_schema_json, permission_level, status, admin_configured, created_by, created_at, updated_at
`, tool.ToolID, tool.Name, tool.Description, tool.ToolType, mcpServerID, tool.MCPToolName, tool.LocalHandlerKey, tool.BuiltinKey,
			tool.InputSchemaJSON, tool.OutputSchemaJSON, tool.PermissionLevel, tool.Status, tool.AdminConfigured, tool.CreatedBy)
	}
	if err != nil {
		if isPostgresForeignKeyViolation(err) {
			return model.AgentTool{}, apperror.NotFound("mcp server not found")
		}
		return model.AgentTool{}, mapAgentRegistryPostgresWriteError(err, "tool already exists", "invalid tool")
	}
	return row.tool(), nil
}

func (r *PostgresRepository) UpsertToolByName(ctx context.Context, tool model.AgentTool) (model.AgentTool, error) {
	mcpServerID := nullableString(tool.MCPServerID)
	var row postgresAgentToolRow
	err := r.conn.QueryRowCtx(ctx, &row, `
insert into agent_tools (
  name, description, tool_type, mcp_server_id, mcp_tool_name, local_handler_key, builtin_key,
  input_schema_json, output_schema_json, permission_level, status, admin_configured, created_by
)
values ($1, $2, $3, $4::bigint, $5, $6, $7, $8::jsonb, $9::jsonb, $10, $11, $12, $13)
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
returning tool_id::text as tool_id, name, description, tool_type, mcp_server_id::text as mcp_server_id, mcp_tool_name, local_handler_key, builtin_key,
          input_schema_json, output_schema_json, permission_level, status, admin_configured, created_by, created_at, updated_at
`, tool.Name, tool.Description, tool.ToolType, mcpServerID, tool.MCPToolName, tool.LocalHandlerKey, tool.BuiltinKey,
		tool.InputSchemaJSON, tool.OutputSchemaJSON, tool.PermissionLevel, tool.Status, tool.AdminConfigured, tool.CreatedBy)
	if err != nil {
		if isPostgresForeignKeyViolation(err) {
			return model.AgentTool{}, apperror.NotFound("mcp server not found")
		}
		return model.AgentTool{}, mapAgentRegistryPostgresWriteError(err, "tool already exists", "invalid tool")
	}
	return row.tool(), nil
}

func (r *PostgresRepository) GetTool(ctx context.Context, toolID string) (model.AgentTool, error) {
	var row postgresAgentToolRow
	err := r.conn.QueryRowCtx(ctx, &row, `
select tool_id::text as tool_id, name, description, tool_type, mcp_server_id::text as mcp_server_id, mcp_tool_name, local_handler_key, builtin_key,
       input_schema_json, output_schema_json, permission_level, status, admin_configured, created_by, created_at, updated_at
from agent_tools
where tool_id = $1::bigint
`, toolID)
	if err != nil {
		if isNotFound(err) {
			return model.AgentTool{}, apperror.NotFound("tool not found")
		}
		return model.AgentTool{}, err
	}
	return row.tool(), nil
}

func (r *PostgresRepository) GetToolByName(ctx context.Context, name string) (model.AgentTool, error) {
	var row postgresAgentToolRow
	err := r.conn.QueryRowCtx(ctx, &row, `
select tool_id::text as tool_id, name, description, tool_type, mcp_server_id::text as mcp_server_id, mcp_tool_name, local_handler_key, builtin_key,
       input_schema_json, output_schema_json, permission_level, status, admin_configured, created_by, created_at, updated_at
from agent_tools
where name = $1
`, name)
	if err != nil {
		if isNotFound(err) {
			return model.AgentTool{}, apperror.NotFound("tool not found")
		}
		return model.AgentTool{}, err
	}
	return row.tool(), nil
}

func (r *PostgresRepository) ListActiveTools(ctx context.Context) ([]model.AgentTool, error) {
	var rows []postgresAgentToolRow
	err := r.conn.QueryRowsCtx(ctx, &rows, `
select tool_id::text as tool_id, name, description, tool_type, mcp_server_id::text as mcp_server_id, mcp_tool_name, local_handler_key, builtin_key,
       input_schema_json, output_schema_json, permission_level, status, admin_configured, created_by, created_at, updated_at
from agent_tools
where status = 'active'
order by name
`)
	if err != nil {
		return nil, err
	}
	tools := make([]model.AgentTool, 0, len(rows))
	for _, row := range rows {
		tools = append(tools, row.tool())
	}
	return tools, nil
}

func (r *PostgresRepository) BindTool(ctx context.Context, binding model.AgentToolBinding) (model.AgentToolBinding, bool, error) {
	existing, err := queryAgentToolBinding(ctx, r.conn, binding.AgentID, binding.ToolID)
	if err == nil {
		return existing.Clone(), false, nil
	}
	if err != nil && !isAgentRegistryNotFound(err) {
		return model.AgentToolBinding{}, false, err
	}

	row, err := insertAgentToolBinding(ctx, r.conn, binding)
	if err != nil {
		if isPostgresUniqueViolation(err) {
			existing, queryErr := queryAgentToolBinding(ctx, r.conn, binding.AgentID, binding.ToolID)
			return existing.Clone(), false, queryErr
		}
		if isPostgresForeignKeyViolation(err) {
			return model.AgentToolBinding{}, false, apperror.NotFound("tool not found")
		}
		return model.AgentToolBinding{}, false, err
	}
	return row.toolBinding(), true, nil
}

func (r *PostgresRepository) GetToolBinding(ctx context.Context, agentID string, toolID string) (model.AgentToolBinding, error) {
	return queryAgentToolBinding(ctx, r.conn, agentID, toolID)
}

func (r *PostgresRepository) ListToolBindings(ctx context.Context, agentID string) ([]model.AgentToolBinding, error) {
	var rows []postgresAgentToolBindingRow
	err := r.conn.QueryRowsCtx(ctx, &rows, `
select agent_id::text as agent_id, tool_id::text as tool_id, created_by, created_at, updated_at
from agent_tool_bindings
where agent_id = $1::bigint
order by tool_id
`, agentID)
	if err != nil {
		return nil, err
	}
	bindings := make([]model.AgentToolBinding, 0, len(rows))
	for _, row := range rows {
		bindings = append(bindings, row.toolBinding())
	}
	return bindings, nil
}

func (r *PostgresRepository) ReplaceToolBindings(ctx context.Context, agentID string, toolIDs []string, createdBy string) ([]model.AgentToolBinding, error) {
	bindings := make([]model.AgentToolBinding, 0, len(toolIDs))
	err := r.withTx(ctx, func(ctx context.Context, session sqlx.Session) error {
		if _, err := session.ExecCtx(ctx, `
delete from agent_tool_bindings
where agent_id = $1::bigint
`, agentID); err != nil {
			return err
		}
		seen := make(map[string]struct{}, len(toolIDs))
		for _, toolID := range toolIDs {
			if _, ok := seen[toolID]; ok {
				continue
			}
			seen[toolID] = struct{}{}
			row, err := insertAgentToolBinding(ctx, session, model.AgentToolBinding{
				AgentID:   agentID,
				ToolID:    toolID,
				CreatedBy: createdBy,
			})
			if err != nil {
				return err
			}
			bindings = append(bindings, row.toolBinding())
		}
		return nil
	})
	if err != nil {
		if isPostgresForeignKeyViolation(err) {
			return nil, apperror.NotFound("tool not found")
		}
		if isPostgresCheckViolation(err) {
			return nil, apperror.InvalidArgument("invalid tool binding")
		}
		return nil, err
	}
	return bindings, nil
}

func (r *PostgresRepository) RegisterSkill(ctx context.Context, skill model.AgentSkill) (model.AgentSkill, error) {
	var row postgresAgentSkillRow
	var err error
	if strings.TrimSpace(skill.SkillID) == "" {
		err = r.conn.QueryRowCtx(ctx, &row, `
insert into agent_skills (
  name, description, version, object_key, sha256, content_type, size_bytes, status, created_by
)
values ($1, $2, $3, $4, $5, $6, $7, $8, $9)
returning skill_id::text as skill_id, name, description, version, object_key, sha256, content_type, size_bytes, status, created_by, created_at, updated_at
`, skill.Name, skill.Description, skill.Version, skill.ObjectKey, skill.SHA256, skill.ContentType, skill.SizeBytes, skill.Status, skill.CreatedBy)
	} else {
		err = r.conn.QueryRowCtx(ctx, &row, `
insert into agent_skills (
  skill_id, name, description, version, object_key, sha256, content_type, size_bytes, status, created_by
)
values ($1::bigint, $2, $3, $4, $5, $6, $7, $8, $9, $10)
returning skill_id::text as skill_id, name, description, version, object_key, sha256, content_type, size_bytes, status, created_by, created_at, updated_at
`, skill.SkillID, skill.Name, skill.Description, skill.Version, skill.ObjectKey, skill.SHA256, skill.ContentType, skill.SizeBytes, skill.Status, skill.CreatedBy)
	}
	if err != nil {
		return model.AgentSkill{}, mapAgentRegistryPostgresWriteError(err, "skill already exists", "invalid skill")
	}
	return row.skill(), nil
}

func (r *PostgresRepository) GetSkill(ctx context.Context, skillID string) (model.AgentSkill, error) {
	var row postgresAgentSkillRow
	err := r.conn.QueryRowCtx(ctx, &row, `
select skill_id::text as skill_id, name, description, version, object_key, sha256, content_type, size_bytes, status, created_by, created_at, updated_at
from agent_skills
where skill_id = $1::bigint
`, skillID)
	if err != nil {
		if isNotFound(err) {
			return model.AgentSkill{}, apperror.NotFound("skill not found")
		}
		return model.AgentSkill{}, err
	}
	return row.skill(), nil
}

func (r *PostgresRepository) BindSkill(ctx context.Context, binding model.AgentSkillBinding) (model.AgentSkillBinding, bool, error) {
	existing, err := queryAgentSkillBinding(ctx, r.conn, binding.AgentID, binding.SkillID)
	if err == nil {
		return existing.Clone(), false, nil
	}
	if err != nil && !isAgentRegistryNotFound(err) {
		return model.AgentSkillBinding{}, false, err
	}

	row, err := insertAgentSkillBinding(ctx, r.conn, binding)
	if err != nil {
		if isPostgresUniqueViolation(err) {
			existing, queryErr := queryAgentSkillBinding(ctx, r.conn, binding.AgentID, binding.SkillID)
			return existing.Clone(), false, queryErr
		}
		if isPostgresForeignKeyViolation(err) {
			return model.AgentSkillBinding{}, false, apperror.NotFound("skill not found")
		}
		return model.AgentSkillBinding{}, false, err
	}
	return row.skillBinding(), true, nil
}

func queryAgentPromptBinding(ctx context.Context, session sqlx.Session, agentID string, promptID string) (model.AgentPromptBinding, error) {
	var row postgresAgentPromptBindingRow
	err := session.QueryRowCtx(ctx, &row, `
select agent_id::text as agent_id, prompt_id::text as prompt_id, created_by, created_at, updated_at
from agent_prompt_bindings
where agent_id = $1 and prompt_id = $2
`, agentID, promptID)
	if err != nil {
		if isNotFound(err) {
			return model.AgentPromptBinding{}, apperror.NotFound("prompt binding not found")
		}
		return model.AgentPromptBinding{}, err
	}
	return row.promptBinding(), nil
}

func insertAgentPromptBinding(ctx context.Context, session sqlx.Session, binding model.AgentPromptBinding) (postgresAgentPromptBindingRow, error) {
	var row postgresAgentPromptBindingRow
	err := session.QueryRowCtx(ctx, &row, `
insert into agent_prompt_bindings (agent_id, prompt_id, created_by)
values ($1::bigint, $2::bigint, $3)
returning agent_id::text as agent_id, prompt_id::text as prompt_id, created_by, created_at, updated_at
`, binding.AgentID, binding.PromptID, binding.CreatedBy)
	return row, err
}

func queryAgentToolBinding(ctx context.Context, session sqlx.Session, agentID string, toolID string) (model.AgentToolBinding, error) {
	var row postgresAgentToolBindingRow
	err := session.QueryRowCtx(ctx, &row, `
select agent_id::text as agent_id, tool_id::text as tool_id, created_by, created_at, updated_at
from agent_tool_bindings
where agent_id = $1 and tool_id = $2
`, agentID, toolID)
	if err != nil {
		if isNotFound(err) {
			return model.AgentToolBinding{}, apperror.NotFound("tool binding not found")
		}
		return model.AgentToolBinding{}, err
	}
	return row.toolBinding(), nil
}

func insertAgentToolBinding(ctx context.Context, session sqlx.Session, binding model.AgentToolBinding) (postgresAgentToolBindingRow, error) {
	var row postgresAgentToolBindingRow
	err := session.QueryRowCtx(ctx, &row, `
insert into agent_tool_bindings (agent_id, tool_id, created_by)
values ($1::bigint, $2::bigint, $3)
returning agent_id::text as agent_id, tool_id::text as tool_id, created_by, created_at, updated_at
`, binding.AgentID, binding.ToolID, binding.CreatedBy)
	return row, err
}

func queryAgentSkillBinding(ctx context.Context, session sqlx.Session, agentID string, skillID string) (model.AgentSkillBinding, error) {
	var row postgresAgentSkillBindingRow
	err := session.QueryRowCtx(ctx, &row, `
select agent_id::text as agent_id, skill_id::text as skill_id, created_by, created_at, updated_at
from agent_skill_bindings
where agent_id = $1 and skill_id = $2
`, agentID, skillID)
	if err != nil {
		if isNotFound(err) {
			return model.AgentSkillBinding{}, apperror.NotFound("skill binding not found")
		}
		return model.AgentSkillBinding{}, err
	}
	return row.skillBinding(), nil
}

func insertAgentSkillBinding(ctx context.Context, session sqlx.Session, binding model.AgentSkillBinding) (postgresAgentSkillBindingRow, error) {
	var row postgresAgentSkillBindingRow
	err := session.QueryRowCtx(ctx, &row, `
insert into agent_skill_bindings (agent_id, skill_id, created_by)
values ($1::bigint, $2::bigint, $3)
returning agent_id::text as agent_id, skill_id::text as skill_id, created_by, created_at, updated_at
`, binding.AgentID, binding.SkillID, binding.CreatedBy)
	return row, err
}

func nullableString(value string) sql.NullString {
	value = strings.TrimSpace(value)
	return sql.NullString{String: value, Valid: value != ""}
}

func mapAgentRegistryPostgresWriteError(err error, duplicateMessage string, checkMessage string) error {
	if isPostgresUniqueViolation(err) {
		return apperror.AlreadyExists(duplicateMessage)
	}
	if isPostgresCheckViolation(err) {
		return apperror.InvalidArgument(checkMessage)
	}
	return err
}

func isAgentRegistryNotFound(err error) bool {
	if isNotFound(err) {
		return true
	}
	appErr := apperror.From(err)
	return appErr != nil && appErr.Code == apperror.CodeNotFound
}

func (row postgresAgentPromptRow) prompt() model.AgentPrompt {
	return model.AgentPrompt{
		PromptID:            row.PromptID,
		Name:                row.Name,
		Description:         row.Description,
		Content:             row.Content,
		VariablesSchemaJSON: string(row.VariablesSchemaJSON),
		Version:             row.Version,
		Status:              model.AgentPromptStatus(row.Status),
		CreatedBy:           row.CreatedBy,
		CreatedAt:           row.CreatedAt,
		UpdatedAt:           row.UpdatedAt,
	}
}

func (row postgresAgentMCPServerRow) mcpServer() model.AgentMCPServer {
	return model.AgentMCPServer{
		ServerID:         row.ServerID,
		Name:             row.Name,
		Transport:        model.AgentMCPTransport(row.Transport),
		URL:              row.URL,
		ConfigJSON:       string(row.ConfigJSON),
		HeadersSecretRef: row.HeadersSecretRef,
		TimeoutSeconds:   row.TimeoutSeconds,
		Status:           model.AgentToolStatus(row.Status),
		AdminConfigured:  row.AdminConfigured,
		CreatedBy:        row.CreatedBy,
		CreatedAt:        row.CreatedAt,
		UpdatedAt:        row.UpdatedAt,
	}
}

func (row postgresAgentToolRow) tool() model.AgentTool {
	return model.AgentTool{
		ToolID:           row.ToolID,
		Name:             row.Name,
		Description:      row.Description,
		ToolType:         model.AgentToolType(row.ToolType),
		MCPServerID:      row.MCPServerID.String,
		MCPToolName:      row.MCPToolName,
		LocalHandlerKey:  row.LocalHandlerKey,
		BuiltinKey:       row.BuiltinKey,
		InputSchemaJSON:  string(row.InputSchemaJSON),
		OutputSchemaJSON: string(row.OutputSchemaJSON),
		PermissionLevel:  row.PermissionLevel,
		Status:           model.AgentToolStatus(row.Status),
		AdminConfigured:  row.AdminConfigured,
		CreatedBy:        row.CreatedBy,
		CreatedAt:        row.CreatedAt,
		UpdatedAt:        row.UpdatedAt,
	}
}

func (row postgresAgentSkillRow) skill() model.AgentSkill {
	return model.AgentSkill{
		SkillID:     row.SkillID,
		Name:        row.Name,
		Description: row.Description,
		Version:     row.Version,
		ObjectKey:   row.ObjectKey,
		SHA256:      row.SHA256,
		ContentType: row.ContentType,
		SizeBytes:   row.SizeBytes,
		Status:      model.AgentSkillStatus(row.Status),
		CreatedBy:   row.CreatedBy,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}
}

func (row postgresAgentPromptBindingRow) promptBinding() model.AgentPromptBinding {
	return model.AgentPromptBinding{
		AgentID:   row.AgentID,
		PromptID:  row.PromptID,
		CreatedBy: row.CreatedBy,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}
}

func (row postgresAgentToolBindingRow) toolBinding() model.AgentToolBinding {
	return model.AgentToolBinding{
		AgentID:   row.AgentID,
		ToolID:    row.ToolID,
		CreatedBy: row.CreatedBy,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}
}

func (row postgresAgentSkillBindingRow) skillBinding() model.AgentSkillBinding {
	return model.AgentSkillBinding{
		AgentID:   row.AgentID,
		SkillID:   row.SkillID,
		CreatedBy: row.CreatedBy,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}
}
