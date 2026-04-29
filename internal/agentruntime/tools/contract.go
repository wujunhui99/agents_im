package tools

import (
	"context"
	"encoding/json"

	"github.com/wujunhui99/agents_im/internal/model"
)

type Registry interface {
	GetMCPServer(ctx context.Context, serverID string) (model.AgentMCPServer, error)
	GetTool(ctx context.Context, toolID string) (model.AgentTool, error)
	GetToolBinding(ctx context.Context, agentID string, toolID string) (model.AgentToolBinding, error)
	ListToolBindings(ctx context.Context, agentID string) ([]model.AgentToolBinding, error)
}

type Provider interface {
	ResolveAgentTools(ctx context.Context, req ResolveAgentToolsRequest) ([]ResolvedTool, error)
	ResolveTool(ctx context.Context, req ResolveToolRequest) (ResolvedTool, error)
}

type ResolveAgentToolsRequest struct {
	AgentID         string
	ToolIDs         []string
	RequireAdapters bool
	RunID           string
	TraceID         string
	RequestID       string
}

type ResolveToolRequest struct {
	AgentID         string
	ToolID          string
	RequireAdapters bool
	RunID           string
	TraceID         string
	RequestID       string
}

type ResolvedTool struct {
	Spec    ToolSpec
	Adapter ToolAdapter
}

func (t ResolvedTool) HasAdapter() bool {
	return t.Adapter != nil
}

type ToolSpec struct {
	ToolID           string              `json:"tool_id"`
	Name             string              `json:"name"`
	Description      string              `json:"description,omitempty"`
	ToolType         model.AgentToolType `json:"tool_type"`
	InputSchemaJSON  string              `json:"input_schema_json"`
	OutputSchemaJSON string              `json:"output_schema_json"`
	PermissionLevel  string              `json:"permission_level"`
	MCP              *MCPToolSpec        `json:"mcp,omitempty"`
	Local            *LocalToolSpec      `json:"local,omitempty"`
	Builtin          *BuiltinToolSpec    `json:"builtin,omitempty"`
}

type MCPToolSpec struct {
	ServerID         string                  `json:"server_id"`
	ServerName       string                  `json:"server_name"`
	Transport        model.AgentMCPTransport `json:"transport"`
	URL              string                  `json:"url"`
	ConfigJSON       string                  `json:"config_json"`
	HeadersSecretRef string                  `json:"headers_secret_ref,omitempty"`
	TimeoutSeconds   int                     `json:"timeout_seconds"`
	ToolName         string                  `json:"tool_name"`
}

type LocalToolSpec struct {
	HandlerKey string `json:"handler_key"`
}

type BuiltinToolSpec struct {
	BuiltinKey string `json:"builtin_key"`
}

type ToolCall struct {
	RunID     string          `json:"run_id,omitempty"`
	AgentID   string          `json:"agent_id"`
	ToolID    string          `json:"tool_id"`
	ToolName  string          `json:"tool_name"`
	InputJSON json.RawMessage `json:"input_json"`
	TraceID   string          `json:"trace_id,omitempty"`
	RequestID string          `json:"request_id,omitempty"`
}

type ToolResult struct {
	OutputJSON json.RawMessage `json:"output_json,omitempty"`
	Content    string          `json:"content,omitempty"`
}

type ToolAdapter interface {
	Spec() ToolSpec
	Invoke(ctx context.Context, call ToolCall) (ToolResult, error)
}

type AdapterCatalog interface {
	LookupToolAdapter(spec ToolSpec) (ToolAdapter, bool, error)
}

type AdapterCatalogFunc func(spec ToolSpec) (ToolAdapter, bool, error)

func (f AdapterCatalogFunc) LookupToolAdapter(spec ToolSpec) (ToolAdapter, bool, error) {
	if f == nil {
		return nil, false, nil
	}
	return f(spec)
}

type ResolutionStatus string

const (
	ResolutionStatusAllowed ResolutionStatus = "allowed"
	ResolutionStatusDenied  ResolutionStatus = "denied"
)

type ResolutionAuditEvent struct {
	RunID     string
	AgentID   string
	ToolID    string
	ToolName  string
	Status    ResolutionStatus
	Reason    string
	TraceID   string
	RequestID string
}

type ResolutionAuditHook interface {
	RecordToolResolution(ctx context.Context, event ResolutionAuditEvent) error
}
