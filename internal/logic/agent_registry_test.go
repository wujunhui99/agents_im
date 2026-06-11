package logic

import (
	"context"
	"testing"

	"github.com/wujunhui99/agents_im/common/share/model"
	"github.com/wujunhui99/agents_im/internal/repository"
	"github.com/wujunhui99/agents_im/pkg/apperror"
)

func TestAgentRegistryPromptToolSkillLifecycle(t *testing.T) {
	ctx := context.Background()
	registry := NewAgentRegistryLogic(repository.NewMemoryAgentRegistryRepository())

	prompt, err := registry.CreatePrompt(ctx, CreateAgentPromptRequest{
		Name:      "Support Agent System Prompt",
		Content:   "Answer from the product support knowledge base.",
		Version:   "v1",
		Status:    model.AgentPromptStatusActive,
		CreatedBy: "usr_admin",
	})
	if err != nil {
		t.Fatal(err)
	}
	if prompt.PromptID == "" || prompt.Content == "" || prompt.CreatedBy != "usr_admin" || prompt.CreatedAt.IsZero() || prompt.UpdatedAt.IsZero() {
		t.Fatalf("prompt metadata not persisted: %+v", prompt)
	}

	promptBinding, created, err := registry.BindPrompt(ctx, BindAgentPromptRequest{
		AgentID:   "agent_support",
		PromptID:  prompt.PromptID,
		CreatedBy: "usr_admin",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !created || promptBinding.AgentID != "agent_support" || promptBinding.PromptID != prompt.PromptID {
		t.Fatalf("prompt binding mismatch: created=%v binding=%+v", created, promptBinding)
	}
	_, created, err = registry.BindPrompt(ctx, BindAgentPromptRequest{
		AgentID:   "agent_support",
		PromptID:  prompt.PromptID,
		CreatedBy: "usr_admin",
	})
	if err != nil {
		t.Fatal(err)
	}
	if created {
		t.Fatal("duplicate prompt binding should be de-duplicated")
	}

	server, err := registry.RegisterMCPServer(ctx, RegisterMCPServerRequest{
		Name:            "Calendar MCP",
		Transport:       model.AgentMCPTransportHTTP,
		URL:             "https://mcp.example.invalid",
		TimeoutSeconds:  10,
		Status:          model.AgentToolStatusActive,
		AdminConfigured: true,
		CreatedBy:       "usr_admin",
	})
	if err != nil {
		t.Fatal(err)
	}
	tool, err := registry.RegisterTool(ctx, RegisterAgentToolRequest{
		Name:            "calendar.search",
		ToolType:        model.AgentToolTypeMCP,
		MCPServerID:     server.ServerID,
		MCPToolName:     "calendar.search",
		Status:          model.AgentToolStatusActive,
		AdminConfigured: true,
		CreatedBy:       "usr_admin",
	})
	if err != nil {
		t.Fatal(err)
	}
	allowed, err := registry.CanAgentUseTool(ctx, "agent_support", tool.ToolID)
	if err != nil {
		t.Fatal(err)
	}
	if allowed {
		t.Fatal("MCP tool should not be usable before an agent whitelist binding exists")
	}

	toolBinding, created, err := registry.BindTool(ctx, BindAgentToolRequest{
		AgentID:   "agent_support",
		ToolID:    tool.ToolID,
		CreatedBy: "usr_admin",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !created || toolBinding.ToolID != tool.ToolID {
		t.Fatalf("tool binding mismatch: created=%v binding=%+v", created, toolBinding)
	}
	allowed, err = registry.CanAgentUseTool(ctx, "agent_support", tool.ToolID)
	if err != nil {
		t.Fatal(err)
	}
	if !allowed {
		t.Fatal("bound active MCP tool should be usable by the whitelisted agent")
	}
	_, created, err = registry.BindTool(ctx, BindAgentToolRequest{
		AgentID:   "agent_support",
		ToolID:    tool.ToolID,
		CreatedBy: "usr_admin",
	})
	if err != nil {
		t.Fatal(err)
	}
	if created {
		t.Fatal("duplicate tool binding should be de-duplicated")
	}

	skill, err := registry.RegisterSkill(ctx, RegisterAgentSkillRequest{
		Name:        "Support Playbook",
		Version:     "2026.04",
		ObjectKey:   "skills/support/versions/2026.04/SKILL.md",
		SHA256:      "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		ContentType: "text/markdown",
		SizeBytes:   2048,
		Status:      model.AgentSkillStatusActive,
		CreatedBy:   "usr_admin",
	})
	if err != nil {
		t.Fatal(err)
	}
	if skill.SkillID == "" || skill.ObjectKey == "" || skill.SHA256 == "" {
		t.Fatalf("skill metadata not persisted: %+v", skill)
	}
	skillBinding, created, err := registry.BindSkill(ctx, BindAgentSkillRequest{
		AgentID:   "agent_support",
		SkillID:   skill.SkillID,
		CreatedBy: "usr_admin",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !created || skillBinding.SkillID != skill.SkillID {
		t.Fatalf("skill binding mismatch: created=%v binding=%+v", created, skillBinding)
	}
	_, created, err = registry.BindSkill(ctx, BindAgentSkillRequest{
		AgentID:   "agent_support",
		SkillID:   skill.SkillID,
		CreatedBy: "usr_admin",
	})
	if err != nil {
		t.Fatal(err)
	}
	if created {
		t.Fatal("duplicate skill binding should be de-duplicated")
	}
}

func TestAgentRegistryRejectsExecutableToolShapes(t *testing.T) {
	ctx := context.Background()
	registry := NewAgentRegistryLogic(repository.NewMemoryAgentRegistryRepository())

	_, err := registry.RegisterMCPServer(ctx, RegisterMCPServerRequest{
		Name:            "stdio server",
		Transport:       "stdio_admin_only",
		URL:             "stdio://calendar",
		TimeoutSeconds:  5,
		Status:          model.AgentToolStatusActive,
		AdminConfigured: true,
		CreatedBy:       "usr_admin",
	})
	assertInvalidArgument(t, err)

	_, err = registry.RegisterTool(ctx, RegisterAgentToolRequest{
		Name:      "shell",
		ToolType:  "shell",
		Status:    model.AgentToolStatusActive,
		CreatedBy: "usr_admin",
	})
	assertInvalidArgument(t, err)

	_, err = registry.RegisterTool(ctx, RegisterAgentToolRequest{
		Name:            "mcp without admin",
		ToolType:        model.AgentToolTypeMCP,
		MCPServerID:     "mcp_srv_missing",
		MCPToolName:     "calendar.search",
		Status:          model.AgentToolStatusActive,
		AdminConfigured: false,
		CreatedBy:       "usr_normal",
	})
	assertInvalidArgument(t, err)

	_, err = registry.RegisterTool(ctx, RegisterAgentToolRequest{
		Name:            "unknown local handler",
		ToolType:        model.AgentToolTypeLocal,
		LocalHandlerKey: "shell.execute",
		Status:          model.AgentToolStatusActive,
		AdminConfigured: true,
		CreatedBy:       "usr_admin",
	})
	assertInvalidArgument(t, err)

	_, err = registry.RegisterTool(ctx, RegisterAgentToolRequest{
		Name:            "local handler with mcp metadata",
		ToolType:        model.AgentToolTypeLocal,
		LocalHandlerKey: model.LocalToolHandlerGetConversationContext,
		MCPServerID:     "mcp_srv_000001",
		Status:          model.AgentToolStatusActive,
		AdminConfigured: true,
		CreatedBy:       "usr_admin",
	})
	assertInvalidArgument(t, err)

	_, err = registry.RegisterTool(ctx, RegisterAgentToolRequest{
		Name:            "builtin with local handler",
		ToolType:        model.AgentToolTypeBuiltin,
		BuiltinKey:      model.BuiltinToolReadSkillFile,
		LocalHandlerKey: model.LocalToolHandlerGetConversationContext,
		Status:          model.AgentToolStatusActive,
		AdminConfigured: true,
		CreatedBy:       "usr_admin",
	})
	assertInvalidArgument(t, err)

	_, err = registry.RegisterTool(ctx, RegisterAgentToolRequest{
		Name:            "unknown builtin",
		ToolType:        model.AgentToolTypeBuiltin,
		BuiltinKey:      "shell.command",
		Status:          model.AgentToolStatusActive,
		AdminConfigured: true,
		CreatedBy:       "usr_admin",
	})
	assertInvalidArgument(t, err)
}

func TestAgentRegistryRejectsInvalidSkillMetadata(t *testing.T) {
	ctx := context.Background()
	registry := NewAgentRegistryLogic(repository.NewMemoryAgentRegistryRepository())

	_, err := registry.RegisterSkill(ctx, RegisterAgentSkillRequest{
		Name:        "missing object key",
		Version:     "v1",
		SHA256:      "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		ContentType: "text/markdown",
		SizeBytes:   10,
		Status:      model.AgentSkillStatusActive,
		CreatedBy:   "usr_admin",
	})
	assertInvalidArgument(t, err)

	_, err = registry.RegisterSkill(ctx, RegisterAgentSkillRequest{
		Name:        "bad sha",
		Version:     "v1",
		ObjectKey:   "skills/bad/SKILL.md",
		SHA256:      "not-a-sha",
		ContentType: "text/markdown",
		SizeBytes:   10,
		Status:      model.AgentSkillStatusActive,
		CreatedBy:   "usr_admin",
	})
	assertInvalidArgument(t, err)

	_, err = registry.RegisterSkill(ctx, RegisterAgentSkillRequest{
		Name:        "zero size",
		Version:     "v1",
		ObjectKey:   "skills/zero/SKILL.md",
		SHA256:      "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		ContentType: "text/markdown",
		Status:      model.AgentSkillStatusActive,
		CreatedBy:   "usr_admin",
	})
	assertInvalidArgument(t, err)
}

func TestAgentRegistryRejectsMissingBindingTargets(t *testing.T) {
	ctx := context.Background()
	registry := NewAgentRegistryLogic(repository.NewMemoryAgentRegistryRepository())

	_, _, err := registry.BindPrompt(ctx, BindAgentPromptRequest{
		AgentID:   "agent_missing_prompt",
		PromptID:  "prompt_missing",
		CreatedBy: "usr_admin",
	})
	assertAppErrorCode(t, err, apperror.CodeNotFound)

	_, _, err = registry.BindTool(ctx, BindAgentToolRequest{
		AgentID:   "agent_missing_tool",
		ToolID:    "tool_missing",
		CreatedBy: "usr_admin",
	})
	assertAppErrorCode(t, err, apperror.CodeNotFound)

	_, _, err = registry.BindSkill(ctx, BindAgentSkillRequest{
		AgentID:   "agent_missing_skill",
		SkillID:   "skill_missing",
		CreatedBy: "usr_admin",
	})
	assertAppErrorCode(t, err, apperror.CodeNotFound)
}

func assertInvalidArgument(t *testing.T, err error) {
	t.Helper()
	assertAppErrorCode(t, err, apperror.CodeInvalidArgument)
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
