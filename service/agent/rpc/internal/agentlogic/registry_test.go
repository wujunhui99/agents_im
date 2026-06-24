package agentlogic

import (
	"context"
	"testing"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/pkg/model"
	"github.com/wujunhui99/agents_im/service/agent/rpc/internal/registry"
)

// 复刻旧 internal/logic/agent_registry_test.go 关键校验，背靠 registry.NewMemoryStore() 测试 fixture（#606）。

func TestAgentRegistryPromptToolSkillLifecycle(t *testing.T) {
	ctx := context.Background()
	reg := NewAgentRegistryLogic(registry.NewMemoryStore())

	prompt, err := reg.CreatePrompt(ctx, CreateAgentPromptRequest{
		Name:      "Support Agent System Prompt",
		Content:   "Answer from the product support knowledge base.",
		Version:   "v1",
		Status:    model.AgentPromptStatusActive,
		CreatedBy: "usr_admin",
	})
	if err != nil {
		t.Fatal(err)
	}
	// 内存 fixture 不填时间戳（DB/goctl 职责，集成测试覆盖），只校验业务字段。
	if prompt.PromptID == "" || prompt.Content == "" || prompt.CreatedBy != "usr_admin" {
		t.Fatalf("prompt metadata not persisted: %+v", prompt)
	}

	_, created, err := reg.BindPrompt(ctx, BindAgentPromptRequest{AgentID: "1001", PromptID: prompt.PromptID, CreatedBy: "usr_admin"})
	if err != nil || !created {
		t.Fatalf("first prompt bind created=%v err=%v", created, err)
	}
	if _, created, err = reg.BindPrompt(ctx, BindAgentPromptRequest{AgentID: "1001", PromptID: prompt.PromptID, CreatedBy: "usr_admin"}); err != nil || created {
		t.Fatalf("duplicate prompt binding should be de-duplicated: created=%v err=%v", created, err)
	}

	server, err := reg.RegisterMCPServer(ctx, RegisterMCPServerRequest{
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
	tool, err := reg.RegisterTool(ctx, RegisterAgentToolRequest{
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

	allowed, err := reg.CanAgentUseTool(ctx, "1001", tool.ToolID)
	if err != nil {
		t.Fatal(err)
	}
	if allowed {
		t.Fatal("MCP tool should not be usable before an agent whitelist binding exists")
	}

	if _, created, err = reg.BindTool(ctx, BindAgentToolRequest{AgentID: "1001", ToolID: tool.ToolID, CreatedBy: "usr_admin"}); err != nil || !created {
		t.Fatalf("first tool bind created=%v err=%v", created, err)
	}
	allowed, err = reg.CanAgentUseTool(ctx, "1001", tool.ToolID)
	if err != nil || !allowed {
		t.Fatalf("bound active MCP tool should be usable: allowed=%v err=%v", allowed, err)
	}
	if _, created, err = reg.BindTool(ctx, BindAgentToolRequest{AgentID: "1001", ToolID: tool.ToolID, CreatedBy: "usr_admin"}); err != nil || created {
		t.Fatalf("duplicate tool binding should be de-duplicated: created=%v err=%v", created, err)
	}

	skill, err := reg.RegisterSkill(ctx, RegisterAgentSkillRequest{
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
	if _, created, err = reg.BindSkill(ctx, BindAgentSkillRequest{AgentID: "1001", SkillID: skill.SkillID, CreatedBy: "usr_admin"}); err != nil || !created {
		t.Fatalf("first skill bind created=%v err=%v", created, err)
	}
	if _, created, err = reg.BindSkill(ctx, BindAgentSkillRequest{AgentID: "1001", SkillID: skill.SkillID, CreatedBy: "usr_admin"}); err != nil || created {
		t.Fatalf("duplicate skill binding should be de-duplicated: created=%v err=%v", created, err)
	}
}

func TestAgentRegistryRejectsExecutableMCPToolWithoutServer(t *testing.T) {
	ctx := context.Background()
	reg := NewAgentRegistryLogic(registry.NewMemoryStore())

	_, err := reg.RegisterTool(ctx, RegisterAgentToolRequest{
		Name:            "calendar.search",
		ToolType:        model.AgentToolTypeMCP,
		MCPToolName:     "calendar.search",
		Status:          model.AgentToolStatusActive,
		AdminConfigured: true,
		CreatedBy:       "usr_admin",
	})
	if apperror.From(err).Code != apperror.CodeInvalidArgument {
		t.Fatalf("mcp tool without server id error = %v, want INVALID_ARGUMENT", err)
	}
}
