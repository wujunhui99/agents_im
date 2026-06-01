package repository

import (
	"context"

	"github.com/wujunhui99/agents_im/common/share/model"
)

type AgentRegistryRepository interface {
	CreatePrompt(ctx context.Context, prompt model.AgentPrompt) (model.AgentPrompt, error)
	GetPrompt(ctx context.Context, promptID string) (model.AgentPrompt, error)
	GetPromptByNameVersion(ctx context.Context, name string, version string) (model.AgentPrompt, error)
	BindPrompt(ctx context.Context, binding model.AgentPromptBinding) (model.AgentPromptBinding, bool, error)
	ListPromptBindings(ctx context.Context, agentID string) ([]model.AgentPromptBinding, error)
	ReplacePromptBindings(ctx context.Context, agentID string, promptIDs []string, createdBy string) ([]model.AgentPromptBinding, error)

	CreateMCPServer(ctx context.Context, server model.AgentMCPServer) (model.AgentMCPServer, error)
	GetMCPServer(ctx context.Context, serverID string) (model.AgentMCPServer, error)

	RegisterTool(ctx context.Context, tool model.AgentTool) (model.AgentTool, error)
	UpsertToolByName(ctx context.Context, tool model.AgentTool) (model.AgentTool, error)
	GetTool(ctx context.Context, toolID string) (model.AgentTool, error)
	GetToolByName(ctx context.Context, name string) (model.AgentTool, error)
	ListActiveTools(ctx context.Context) ([]model.AgentTool, error)
	BindTool(ctx context.Context, binding model.AgentToolBinding) (model.AgentToolBinding, bool, error)
	GetToolBinding(ctx context.Context, agentID string, toolID string) (model.AgentToolBinding, error)
	ListToolBindings(ctx context.Context, agentID string) ([]model.AgentToolBinding, error)
	ReplaceToolBindings(ctx context.Context, agentID string, toolIDs []string, createdBy string) ([]model.AgentToolBinding, error)

	RegisterSkill(ctx context.Context, skill model.AgentSkill) (model.AgentSkill, error)
	GetSkill(ctx context.Context, skillID string) (model.AgentSkill, error)
	BindSkill(ctx context.Context, binding model.AgentSkillBinding) (model.AgentSkillBinding, bool, error)
}
