// Package registry 是 agent 注册表(prompts/tools/skills/mcp + bindings)的 agent 自有
// goctl model 数据层(#605:数据层脱 internal/repository → service/agent/rpc/internal/model)。
//
// 边界口径:注册表列在 #013 已迁 bigint(prompt_id/tool_id/server_id/agent_id 等),但
// proto / pkg/model / runtime orchestrator 全用 string ID。本 Store 在内部做 string↔int64
// 转换,对外只暴露 string ID(沿用 bigint keystone 的既有约定),消费方无需改类型。
//
// 本 Store 仅覆盖 agent-rpc 进程内只读消费方(orchestrator 请求构建 + runtime tool 解析)
// 所需的读方法;注册表写/校验仍由 internal keystone(AgentAssemblyLogic/DefaultAssistant)
// 承担,待后续 PR(saga 拆解 CreateAgentFromTool)随 internal 退役一并迁出。
package registry

import (
	"context"
	"strconv"

	"github.com/zeromicro/go-zero/core/stores/postgres"
	"github.com/zeromicro/go-zero/core/stores/sqlx"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/pkg/model"
	rpcmodel "github.com/wujunhui99/agents_im/service/agent/rpc/internal/model"
)

// Reader 是注册表只读视图,是 runtime tool 解析器(tools.Registry)与 orchestrator 请求
// 构建器所需读方法的并集,故同一个 *Store 值可同时喂这两个消费方(避免 nil-interface 陷阱)。
type Reader interface {
	GetPrompt(ctx context.Context, promptID string) (model.AgentPrompt, error)
	ListPromptBindings(ctx context.Context, agentID string) ([]model.AgentPromptBinding, error)
	GetTool(ctx context.Context, toolID string) (model.AgentTool, error)
	GetToolBinding(ctx context.Context, agentID string, toolID string) (model.AgentToolBinding, error)
	ListToolBindings(ctx context.Context, agentID string) ([]model.AgentToolBinding, error)
	GetMCPServer(ctx context.Context, serverID string) (model.AgentMCPServer, error)
}

// Store 是 Reader 的 goctl model 实现。
type Store struct {
	prompts        rpcmodel.AgentPromptsModel
	tools          rpcmodel.AgentToolsModel
	mcpServers     rpcmodel.McpServersModel
	promptBindings rpcmodel.AgentPromptBindingsModel
	toolBindings   rpcmodel.AgentToolBindingsModel
}

var _ Reader = (*Store)(nil)

// NewStore 用数据源构建 model-backed 注册表只读 Store。
func NewStore(dataSource string) *Store {
	return NewStoreFromConn(postgres.New(dataSource))
}

// NewStoreFromConn 用已建连接构建 model-backed 注册表只读 Store。
func NewStoreFromConn(conn sqlx.SqlConn) *Store {
	return &Store{
		prompts:        rpcmodel.NewAgentPromptsModel(conn),
		tools:          rpcmodel.NewAgentToolsModel(conn),
		mcpServers:     rpcmodel.NewMcpServersModel(conn),
		promptBindings: rpcmodel.NewAgentPromptBindingsModel(conn),
		toolBindings:   rpcmodel.NewAgentToolBindingsModel(conn),
	}
}

func (s *Store) GetPrompt(ctx context.Context, promptID string) (model.AgentPrompt, error) {
	id, ok := parseID(promptID)
	if !ok {
		return model.AgentPrompt{}, apperror.NotFound("prompt not found")
	}
	row, err := s.prompts.FindOne(ctx, id)
	if err != nil {
		return model.AgentPrompt{}, mapNotFound(err, "prompt not found")
	}
	return promptFromModel(row), nil
}

func (s *Store) ListPromptBindings(ctx context.Context, agentID string) ([]model.AgentPromptBinding, error) {
	id, ok := parseID(agentID)
	if !ok {
		return nil, nil
	}
	rows, err := s.promptBindings.FindByAgentId(ctx, id)
	if err != nil {
		return nil, err
	}
	bindings := make([]model.AgentPromptBinding, 0, len(rows))
	for _, row := range rows {
		bindings = append(bindings, model.AgentPromptBinding{
			AgentID:   formatID(row.AgentId),
			PromptID:  formatID(row.PromptId),
			CreatedBy: row.CreatedBy,
			CreatedAt: row.CreatedAt,
			UpdatedAt: row.UpdatedAt,
		})
	}
	return bindings, nil
}

func (s *Store) GetTool(ctx context.Context, toolID string) (model.AgentTool, error) {
	id, ok := parseID(toolID)
	if !ok {
		return model.AgentTool{}, apperror.NotFound("tool not found")
	}
	row, err := s.tools.FindOne(ctx, id)
	if err != nil {
		return model.AgentTool{}, mapNotFound(err, "tool not found")
	}
	return toolFromModel(row), nil
}

func (s *Store) GetToolBinding(ctx context.Context, agentID string, toolID string) (model.AgentToolBinding, error) {
	aid, aok := parseID(agentID)
	tid, tok := parseID(toolID)
	if !aok || !tok {
		return model.AgentToolBinding{}, apperror.NotFound("tool binding not found")
	}
	row, err := s.toolBindings.FindOneByAgentIdToolId(ctx, aid, tid)
	if err != nil {
		return model.AgentToolBinding{}, mapNotFound(err, "tool binding not found")
	}
	return model.AgentToolBinding{
		AgentID:   formatID(row.AgentId),
		ToolID:    formatID(row.ToolId),
		CreatedBy: row.CreatedBy,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}, nil
}

func (s *Store) ListToolBindings(ctx context.Context, agentID string) ([]model.AgentToolBinding, error) {
	id, ok := parseID(agentID)
	if !ok {
		return nil, nil
	}
	rows, err := s.toolBindings.FindByAgentId(ctx, id)
	if err != nil {
		return nil, err
	}
	bindings := make([]model.AgentToolBinding, 0, len(rows))
	for _, row := range rows {
		bindings = append(bindings, model.AgentToolBinding{
			AgentID:   formatID(row.AgentId),
			ToolID:    formatID(row.ToolId),
			CreatedBy: row.CreatedBy,
			CreatedAt: row.CreatedAt,
			UpdatedAt: row.UpdatedAt,
		})
	}
	return bindings, nil
}

func (s *Store) GetMCPServer(ctx context.Context, serverID string) (model.AgentMCPServer, error) {
	id, ok := parseID(serverID)
	if !ok {
		return model.AgentMCPServer{}, apperror.NotFound("mcp server not found")
	}
	row, err := s.mcpServers.FindOne(ctx, id)
	if err != nil {
		return model.AgentMCPServer{}, mapNotFound(err, "mcp server not found")
	}
	return model.AgentMCPServer{
		ServerID:         formatID(row.ServerId),
		Name:             row.Name,
		Transport:        model.AgentMCPTransport(row.Transport),
		URL:              row.Url,
		ConfigJSON:       row.ConfigJson,
		HeadersSecretRef: row.HeadersSecretRef,
		TimeoutSeconds:   int(row.TimeoutSeconds),
		Status:           model.AgentToolStatus(row.Status),
		AdminConfigured:  row.AdminConfigured,
		CreatedBy:        row.CreatedBy,
		CreatedAt:        row.CreatedAt,
		UpdatedAt:        row.UpdatedAt,
	}, nil
}

func promptFromModel(row *rpcmodel.AgentPrompts) model.AgentPrompt {
	return model.AgentPrompt{
		PromptID:            formatID(row.PromptId),
		Name:                row.Name,
		Description:         row.Description,
		Content:             row.Content,
		VariablesSchemaJSON: row.VariablesSchemaJson,
		Version:             row.Version,
		Status:              model.AgentPromptStatus(row.Status),
		CreatedBy:           row.CreatedBy,
		CreatedAt:           row.CreatedAt,
		UpdatedAt:           row.UpdatedAt,
	}
}

func toolFromModel(row *rpcmodel.AgentTools) model.AgentTool {
	mcpServerID := ""
	if row.McpServerId.Valid {
		mcpServerID = formatID(row.McpServerId.Int64)
	}
	return model.AgentTool{
		ToolID:           formatID(row.ToolId),
		Name:             row.Name,
		Description:      row.Description,
		ToolType:         model.AgentToolType(row.ToolType),
		MCPServerID:      mcpServerID,
		MCPToolName:      row.McpToolName,
		LocalHandlerKey:  row.LocalHandlerKey,
		BuiltinKey:       row.BuiltinKey,
		InputSchemaJSON:  row.InputSchemaJson,
		OutputSchemaJSON: row.OutputSchemaJson,
		PermissionLevel:  row.PermissionLevel,
		Status:           model.AgentToolStatus(row.Status),
		AdminConfigured:  row.AdminConfigured,
		CreatedBy:        row.CreatedBy,
		CreatedAt:        row.CreatedAt,
		UpdatedAt:        row.UpdatedAt,
	}
}

func parseID(value string) (int64, bool) {
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, false
	}
	return id, true
}

func formatID(value int64) string {
	return strconv.FormatInt(value, 10)
}

func mapNotFound(err error, message string) error {
	if err == rpcmodel.ErrNotFound {
		return apperror.NotFound(message)
	}
	return err
}
