package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/wujunhui99/agents_im/internal/apperror"
	"github.com/wujunhui99/agents_im/internal/model"
	"github.com/wujunhui99/agents_im/internal/repository"
)

func TestResolverResolvesAllowedBoundMCPTool(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewMemoryAgentRegistryRepository()
	seedMCPTool(t, ctx, repo, seedMCPToolInput{
		AgentID: "agent_support",
		ToolID:  "tool_calendar",
	})
	resolver := newResolver(t, repo)

	resolved, err := resolver.ResolveTool(ctx, ResolveToolRequest{
		AgentID: "agent_support",
		ToolID:  "tool_calendar",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Spec.ToolID != "tool_calendar" || resolved.Spec.Name != "calendar.search" {
		t.Fatalf("tool spec mismatch: %+v", resolved.Spec)
	}
	if resolved.Spec.MCP == nil {
		t.Fatalf("expected mcp spec: %+v", resolved.Spec)
	}
	if resolved.Spec.MCP.Transport != model.AgentMCPTransportHTTP || resolved.Spec.MCP.URL != "https://mcp.example.invalid" {
		t.Fatalf("mcp server spec mismatch: %+v", resolved.Spec.MCP)
	}
	if resolved.HasAdapter() {
		t.Fatal("metadata-only resolution should not invent an adapter")
	}

	allowed, err := resolver.ResolveAgentTools(ctx, ResolveAgentToolsRequest{AgentID: "agent_support"})
	if err != nil {
		t.Fatal(err)
	}
	if len(allowed) != 1 || allowed[0].Spec.ToolID != "tool_calendar" {
		t.Fatalf("allowed tool list mismatch: %+v", allowed)
	}
}

func TestResolverRejectsMissingBinding(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewMemoryAgentRegistryRepository()
	seedMCPTool(t, ctx, repo, seedMCPToolInput{
		AgentID: "other_agent",
		ToolID:  "tool_calendar",
	})
	resolver := newResolver(t, repo)

	_, err := resolver.ResolveTool(ctx, ResolveToolRequest{
		AgentID: "agent_support",
		ToolID:  "tool_calendar",
	})
	assertAppErrorCode(t, err, apperror.CodeForbidden)
}

func TestResolverRejectsDisabledAndArchivedTools(t *testing.T) {
	tests := []struct {
		name   string
		status model.AgentToolStatus
	}{
		{name: "disabled", status: model.AgentToolStatusDisabled},
		{name: "archived", status: model.AgentToolStatusArchived},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			repo := repository.NewMemoryAgentRegistryRepository()
			seedMCPTool(t, ctx, repo, seedMCPToolInput{
				AgentID:    "agent_support",
				ToolID:     "tool_calendar",
				ToolStatus: tt.status,
			})
			resolver := newResolver(t, repo)

			_, err := resolver.ResolveTool(ctx, ResolveToolRequest{
				AgentID: "agent_support",
				ToolID:  "tool_calendar",
			})
			assertAppErrorCode(t, err, apperror.CodeForbidden)
		})
	}
}

func TestResolverRejectsNonAdminMCPPolicy(t *testing.T) {
	tests := []struct {
		name             string
		toolAdmin        bool
		mcpServerAdmin   bool
		wantErrorMessage string
	}{
		{
			name:             "tool not admin configured",
			toolAdmin:        false,
			mcpServerAdmin:   true,
			wantErrorMessage: "admin",
		},
		{
			name:             "server not admin configured",
			toolAdmin:        true,
			mcpServerAdmin:   false,
			wantErrorMessage: "admin",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			repo := repository.NewMemoryAgentRegistryRepository()
			seedMCPTool(t, ctx, repo, seedMCPToolInput{
				AgentID:          "agent_support",
				ToolID:           "tool_calendar",
				ToolAdmin:        &tt.toolAdmin,
				MCPServerAdmin:   &tt.mcpServerAdmin,
				MCPServerStatus:  model.AgentToolStatusActive,
				MCPServerTimeout: 10,
			})
			resolver := newResolver(t, repo)

			_, err := resolver.ResolveTool(ctx, ResolveToolRequest{
				AgentID: "agent_support",
				ToolID:  "tool_calendar",
			})
			assertAppErrorCode(t, err, apperror.CodeForbidden)
			if err == nil || !strings.Contains(err.Error(), tt.wantErrorMessage) {
				t.Fatalf("expected error containing %q, got %v", tt.wantErrorMessage, err)
			}
		})
	}
}

func TestResolverRejectsUnsafeMCPTransportAndProcessMetadata(t *testing.T) {
	tests := []struct {
		name      string
		transport model.AgentMCPTransport
		config    string
	}{
		{
			name:      "stdio transport",
			transport: model.AgentMCPTransport("stdio"),
			config:    `{}`,
		},
		{
			name:      "process command metadata",
			transport: model.AgentMCPTransportHTTP,
			config:    `{"command":"node server.js"}`,
		},
		{
			name:      "nested stdio config",
			transport: model.AgentMCPTransportHTTP,
			config:    `{"runtime":{"transport":"stdio"}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			repo := repository.NewMemoryAgentRegistryRepository()
			seedMCPTool(t, ctx, repo, seedMCPToolInput{
				AgentID:            "agent_support",
				ToolID:             "tool_calendar",
				MCPServerTransport: tt.transport,
				MCPServerConfig:    tt.config,
			})
			resolver := newResolver(t, repo)

			_, err := resolver.ResolveTool(ctx, ResolveToolRequest{
				AgentID: "agent_support",
				ToolID:  "tool_calendar",
			})
			assertAppErrorCode(t, err, apperror.CodeForbidden)
		})
	}
}

func TestResolverLocalToolMetadataFailsClosedWithoutAdapter(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewMemoryAgentRegistryRepository()
	seedLocalTool(t, ctx, repo, "agent_support", "tool_context")
	resolver := newResolver(t, repo)

	resolved, err := resolver.ResolveTool(ctx, ResolveToolRequest{
		AgentID: "agent_support",
		ToolID:  "tool_context",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Spec.Local == nil || resolved.HasAdapter() {
		t.Fatalf("expected metadata-only local tool spec: %+v", resolved)
	}

	_, err = resolver.ResolveTool(ctx, ResolveToolRequest{
		AgentID:         "agent_support",
		ToolID:          "tool_context",
		RequireAdapters: true,
	})
	assertAppErrorCode(t, err, apperror.CodeForbidden)

	resolverWithCatalog, err := NewResolver(repo, WithAdapterCatalog(AdapterCatalogFunc(func(spec ToolSpec) (ToolAdapter, bool, error) {
		return fakeAdapter{spec: spec}, true, nil
	})))
	if err != nil {
		t.Fatal(err)
	}
	resolved, err = resolverWithCatalog.ResolveTool(ctx, ResolveToolRequest{
		AgentID:         "agent_support",
		ToolID:          "tool_context",
		RequireAdapters: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !resolved.HasAdapter() {
		t.Fatal("expected explicit safe adapter")
	}
}

type seedMCPToolInput struct {
	AgentID            string
	ToolID             string
	ToolStatus         model.AgentToolStatus
	ToolAdmin          *bool
	MCPServerStatus    model.AgentToolStatus
	MCPServerAdmin     *bool
	MCPServerTransport model.AgentMCPTransport
	MCPServerTimeout   int
	MCPServerConfig    string
}

func seedMCPTool(t *testing.T, ctx context.Context, repo *repository.MemoryAgentRegistryRepository, input seedMCPToolInput) {
	t.Helper()
	toolStatus := input.ToolStatus
	if toolStatus == "" {
		toolStatus = model.AgentToolStatusActive
	}
	toolAdmin := true
	if input.ToolAdmin != nil {
		toolAdmin = *input.ToolAdmin
	}
	serverStatus := input.MCPServerStatus
	if serverStatus == "" {
		serverStatus = model.AgentToolStatusActive
	}
	serverAdmin := true
	if input.MCPServerAdmin != nil {
		serverAdmin = *input.MCPServerAdmin
	}
	transport := input.MCPServerTransport
	if transport == "" {
		transport = model.AgentMCPTransportHTTP
	}
	timeout := input.MCPServerTimeout
	if timeout == 0 {
		timeout = 10
	}
	config := input.MCPServerConfig
	if config == "" {
		config = `{}`
	}

	_, err := repo.CreateMCPServer(ctx, model.AgentMCPServer{
		ServerID:        "mcp_calendar",
		Name:            "Calendar MCP",
		Transport:       transport,
		URL:             "https://mcp.example.invalid",
		ConfigJSON:      config,
		TimeoutSeconds:  timeout,
		Status:          serverStatus,
		AdminConfigured: serverAdmin,
		CreatedBy:       "usr_admin",
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = repo.RegisterTool(ctx, model.AgentTool{
		ToolID:           input.ToolID,
		Name:             "calendar.search",
		ToolType:         model.AgentToolTypeMCP,
		MCPServerID:      "mcp_calendar",
		MCPToolName:      "calendar.search",
		InputSchemaJSON:  `{"type":"object"}`,
		OutputSchemaJSON: `{"type":"object"}`,
		PermissionLevel:  "agent_bound",
		Status:           toolStatus,
		AdminConfigured:  toolAdmin,
		CreatedBy:        "usr_admin",
	})
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = repo.BindTool(ctx, model.AgentToolBinding{
		AgentID:   input.AgentID,
		ToolID:    input.ToolID,
		CreatedBy: "usr_admin",
	})
	if err != nil {
		t.Fatal(err)
	}
}

func seedLocalTool(t *testing.T, ctx context.Context, repo *repository.MemoryAgentRegistryRepository, agentID string, toolID string) {
	t.Helper()
	_, err := repo.RegisterTool(ctx, model.AgentTool{
		ToolID:           toolID,
		Name:             model.LocalToolHandlerGetConversationContext,
		ToolType:         model.AgentToolTypeLocal,
		LocalHandlerKey:  model.LocalToolHandlerGetConversationContext,
		InputSchemaJSON:  `{"type":"object"}`,
		OutputSchemaJSON: `{"type":"object"}`,
		PermissionLevel:  "agent_bound",
		Status:           model.AgentToolStatusActive,
		AdminConfigured:  true,
		CreatedBy:        "usr_admin",
	})
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = repo.BindTool(ctx, model.AgentToolBinding{
		AgentID:   agentID,
		ToolID:    toolID,
		CreatedBy: "usr_admin",
	})
	if err != nil {
		t.Fatal(err)
	}
}

func newResolver(t *testing.T, repo *repository.MemoryAgentRegistryRepository) *Resolver {
	t.Helper()
	resolver, err := NewResolver(repo)
	if err != nil {
		t.Fatal(err)
	}
	return resolver
}

type fakeAdapter struct {
	spec ToolSpec
}

func (a fakeAdapter) Spec() ToolSpec {
	return a.spec
}

func (a fakeAdapter) Invoke(context.Context, ToolCall) (ToolResult, error) {
	return ToolResult{OutputJSON: json.RawMessage(`{}`)}, nil
}

func assertAppErrorCode(t *testing.T, err error, code apperror.Code) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected %s error", code)
	}
	if appErr := apperror.From(err); appErr.Code != code {
		t.Fatalf("expected %s error, got %v", code, err)
	}
}
