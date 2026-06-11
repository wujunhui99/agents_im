package repository

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/wujunhui99/agents_im/common/share/model"
	"github.com/wujunhui99/agents_im/pkg/apperror"
)

type MemoryAgentRegistryRepository struct {
	mu sync.RWMutex

	nextPromptID uint64
	nextServerID uint64
	nextToolID   uint64
	nextSkillID  uint64

	prompts     map[string]model.AgentPrompt
	mcpServers  map[string]model.AgentMCPServer
	tools       map[string]model.AgentTool
	skills      map[string]model.AgentSkill
	promptBinds map[string]model.AgentPromptBinding
	toolBinds   map[string]model.AgentToolBinding
	skillBinds  map[string]model.AgentSkillBinding
	now         func() time.Time
}

func NewMemoryAgentRegistryRepository() *MemoryAgentRegistryRepository {
	return &MemoryAgentRegistryRepository{
		prompts:     make(map[string]model.AgentPrompt),
		mcpServers:  make(map[string]model.AgentMCPServer),
		tools:       make(map[string]model.AgentTool),
		skills:      make(map[string]model.AgentSkill),
		promptBinds: make(map[string]model.AgentPromptBinding),
		toolBinds:   make(map[string]model.AgentToolBinding),
		skillBinds:  make(map[string]model.AgentSkillBinding),
		now:         time.Now,
	}
}

func (r *MemoryAgentRegistryRepository) CreatePrompt(_ context.Context, prompt model.AgentPrompt) (model.AgentPrompt, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if prompt.PromptID == "" {
		r.nextPromptID++
		prompt.PromptID = fmt.Sprintf("prompt_%06d", r.nextPromptID)
	} else if _, exists := r.prompts[prompt.PromptID]; exists {
		return model.AgentPrompt{}, apperror.AlreadyExists("prompt already exists")
	}
	now := r.now().UTC()
	prompt.CreatedAt = now
	prompt.UpdatedAt = now

	r.prompts[prompt.PromptID] = prompt.Clone()
	return prompt.Clone(), nil
}

func (r *MemoryAgentRegistryRepository) GetPrompt(_ context.Context, promptID string) (model.AgentPrompt, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	prompt, exists := r.prompts[promptID]
	if !exists {
		return model.AgentPrompt{}, apperror.NotFound("prompt not found")
	}
	return prompt.Clone(), nil
}

func (r *MemoryAgentRegistryRepository) GetPromptByNameVersion(_ context.Context, name string, version string) (model.AgentPrompt, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, prompt := range r.prompts {
		if prompt.Name == name && prompt.Version == version {
			return prompt.Clone(), nil
		}
	}
	return model.AgentPrompt{}, apperror.NotFound("prompt not found")
}

func (r *MemoryAgentRegistryRepository) BindPrompt(_ context.Context, binding model.AgentPromptBinding) (model.AgentPromptBinding, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := promptBindingKey(binding.AgentID, binding.PromptID)
	if existing, exists := r.promptBinds[key]; exists {
		return existing.Clone(), false, nil
	}
	now := r.now().UTC()
	binding.CreatedAt = now
	binding.UpdatedAt = now
	r.promptBinds[key] = binding.Clone()
	return binding.Clone(), true, nil
}

func (r *MemoryAgentRegistryRepository) ListPromptBindings(_ context.Context, agentID string) ([]model.AgentPromptBinding, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	bindings := make([]model.AgentPromptBinding, 0)
	for _, binding := range r.promptBinds {
		if binding.AgentID == agentID {
			bindings = append(bindings, binding.Clone())
		}
	}
	sort.Slice(bindings, func(i, j int) bool {
		if bindings[i].CreatedAt.Equal(bindings[j].CreatedAt) {
			return bindings[i].PromptID < bindings[j].PromptID
		}
		return bindings[i].CreatedAt.After(bindings[j].CreatedAt)
	})
	return bindings, nil
}

func (r *MemoryAgentRegistryRepository) ReplacePromptBindings(_ context.Context, agentID string, promptIDs []string, createdBy string) ([]model.AgentPromptBinding, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	seen := make(map[string]struct{}, len(promptIDs))
	for _, promptID := range promptIDs {
		if _, ok := seen[promptID]; ok {
			continue
		}
		seen[promptID] = struct{}{}
		if _, exists := r.prompts[promptID]; !exists {
			return nil, apperror.NotFound("prompt not found")
		}
	}
	for key, binding := range r.promptBinds {
		if binding.AgentID == agentID {
			delete(r.promptBinds, key)
		}
	}

	now := r.now().UTC()
	bindings := make([]model.AgentPromptBinding, 0, len(promptIDs))
	seen = make(map[string]struct{}, len(promptIDs))
	for _, promptID := range promptIDs {
		if _, ok := seen[promptID]; ok {
			continue
		}
		seen[promptID] = struct{}{}
		binding := model.AgentPromptBinding{
			AgentID:   agentID,
			PromptID:  promptID,
			CreatedBy: createdBy,
			CreatedAt: now,
			UpdatedAt: now,
		}
		r.promptBinds[promptBindingKey(agentID, promptID)] = binding.Clone()
		bindings = append(bindings, binding.Clone())
	}
	return bindings, nil
}

func (r *MemoryAgentRegistryRepository) CreateMCPServer(_ context.Context, server model.AgentMCPServer) (model.AgentMCPServer, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if server.ServerID == "" {
		r.nextServerID++
		server.ServerID = fmt.Sprintf("mcp_srv_%06d", r.nextServerID)
	} else if _, exists := r.mcpServers[server.ServerID]; exists {
		return model.AgentMCPServer{}, apperror.AlreadyExists("mcp server already exists")
	}
	now := r.now().UTC()
	server.CreatedAt = now
	server.UpdatedAt = now

	r.mcpServers[server.ServerID] = server.Clone()
	return server.Clone(), nil
}

func (r *MemoryAgentRegistryRepository) GetMCPServer(_ context.Context, serverID string) (model.AgentMCPServer, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	server, exists := r.mcpServers[serverID]
	if !exists {
		return model.AgentMCPServer{}, apperror.NotFound("mcp server not found")
	}
	return server.Clone(), nil
}

func (r *MemoryAgentRegistryRepository) RegisterTool(_ context.Context, tool model.AgentTool) (model.AgentTool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if tool.ToolID == "" {
		r.nextToolID++
		tool.ToolID = fmt.Sprintf("tool_%06d", r.nextToolID)
	} else if _, exists := r.tools[tool.ToolID]; exists {
		return model.AgentTool{}, apperror.AlreadyExists("tool already exists")
	}
	now := r.now().UTC()
	tool.CreatedAt = now
	tool.UpdatedAt = now

	r.tools[tool.ToolID] = tool.Clone()
	return tool.Clone(), nil
}

func (r *MemoryAgentRegistryRepository) UpsertToolByName(_ context.Context, tool model.AgentTool) (model.AgentTool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := r.now().UTC()
	for _, existing := range r.tools {
		if existing.Name != tool.Name {
			continue
		}
		tool.ToolID = existing.ToolID
		tool.CreatedAt = existing.CreatedAt
		tool.UpdatedAt = now
		r.tools[tool.ToolID] = tool.Clone()
		return tool.Clone(), nil
	}

	if tool.ToolID == "" {
		r.nextToolID++
		tool.ToolID = fmt.Sprintf("tool_%06d", r.nextToolID)
	} else if _, exists := r.tools[tool.ToolID]; exists {
		return model.AgentTool{}, apperror.AlreadyExists("tool already exists")
	}
	tool.CreatedAt = now
	tool.UpdatedAt = now
	r.tools[tool.ToolID] = tool.Clone()
	return tool.Clone(), nil
}

func (r *MemoryAgentRegistryRepository) GetTool(_ context.Context, toolID string) (model.AgentTool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tool, exists := r.tools[toolID]
	if !exists {
		return model.AgentTool{}, apperror.NotFound("tool not found")
	}
	return tool.Clone(), nil
}

func (r *MemoryAgentRegistryRepository) GetToolByName(_ context.Context, name string) (model.AgentTool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, tool := range r.tools {
		if tool.Name == name {
			return tool.Clone(), nil
		}
	}
	return model.AgentTool{}, apperror.NotFound("tool not found")
}

func (r *MemoryAgentRegistryRepository) ListActiveTools(_ context.Context) ([]model.AgentTool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tools := make([]model.AgentTool, 0)
	for _, tool := range r.tools {
		if tool.Status == model.AgentToolStatusActive {
			tools = append(tools, tool.Clone())
		}
	}
	sort.Slice(tools, func(i, j int) bool {
		return tools[i].Name < tools[j].Name
	})
	return tools, nil
}

func (r *MemoryAgentRegistryRepository) BindTool(_ context.Context, binding model.AgentToolBinding) (model.AgentToolBinding, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := toolBindingKey(binding.AgentID, binding.ToolID)
	if existing, exists := r.toolBinds[key]; exists {
		return existing.Clone(), false, nil
	}
	now := r.now().UTC()
	binding.CreatedAt = now
	binding.UpdatedAt = now
	r.toolBinds[key] = binding.Clone()
	return binding.Clone(), true, nil
}

func (r *MemoryAgentRegistryRepository) GetToolBinding(_ context.Context, agentID string, toolID string) (model.AgentToolBinding, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	binding, exists := r.toolBinds[toolBindingKey(agentID, toolID)]
	if !exists {
		return model.AgentToolBinding{}, apperror.NotFound("tool binding not found")
	}
	return binding.Clone(), nil
}

func (r *MemoryAgentRegistryRepository) ListToolBindings(_ context.Context, agentID string) ([]model.AgentToolBinding, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	bindings := make([]model.AgentToolBinding, 0)
	for _, binding := range r.toolBinds {
		if binding.AgentID == agentID {
			bindings = append(bindings, binding.Clone())
		}
	}
	sort.Slice(bindings, func(i, j int) bool {
		return bindings[i].ToolID < bindings[j].ToolID
	})
	return bindings, nil
}

func (r *MemoryAgentRegistryRepository) ReplaceToolBindings(_ context.Context, agentID string, toolIDs []string, createdBy string) ([]model.AgentToolBinding, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	seen := make(map[string]struct{}, len(toolIDs))
	for _, toolID := range toolIDs {
		if _, ok := seen[toolID]; ok {
			continue
		}
		seen[toolID] = struct{}{}
		if _, exists := r.tools[toolID]; !exists {
			return nil, apperror.NotFound("tool not found")
		}
	}
	for key, binding := range r.toolBinds {
		if binding.AgentID == agentID {
			delete(r.toolBinds, key)
		}
	}

	now := r.now().UTC()
	bindings := make([]model.AgentToolBinding, 0, len(toolIDs))
	seen = make(map[string]struct{}, len(toolIDs))
	for _, toolID := range toolIDs {
		if _, ok := seen[toolID]; ok {
			continue
		}
		seen[toolID] = struct{}{}
		binding := model.AgentToolBinding{
			AgentID:   agentID,
			ToolID:    toolID,
			CreatedBy: createdBy,
			CreatedAt: now,
			UpdatedAt: now,
		}
		r.toolBinds[toolBindingKey(agentID, toolID)] = binding.Clone()
		bindings = append(bindings, binding.Clone())
	}
	return bindings, nil
}

func (r *MemoryAgentRegistryRepository) RegisterSkill(_ context.Context, skill model.AgentSkill) (model.AgentSkill, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if skill.SkillID == "" {
		r.nextSkillID++
		skill.SkillID = fmt.Sprintf("skill_%06d", r.nextSkillID)
	} else if _, exists := r.skills[skill.SkillID]; exists {
		return model.AgentSkill{}, apperror.AlreadyExists("skill already exists")
	}
	now := r.now().UTC()
	skill.CreatedAt = now
	skill.UpdatedAt = now

	r.skills[skill.SkillID] = skill.Clone()
	return skill.Clone(), nil
}

func (r *MemoryAgentRegistryRepository) GetSkill(_ context.Context, skillID string) (model.AgentSkill, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	skill, exists := r.skills[skillID]
	if !exists {
		return model.AgentSkill{}, apperror.NotFound("skill not found")
	}
	return skill.Clone(), nil
}

func (r *MemoryAgentRegistryRepository) BindSkill(_ context.Context, binding model.AgentSkillBinding) (model.AgentSkillBinding, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := skillBindingKey(binding.AgentID, binding.SkillID)
	if existing, exists := r.skillBinds[key]; exists {
		return existing.Clone(), false, nil
	}
	now := r.now().UTC()
	binding.CreatedAt = now
	binding.UpdatedAt = now
	r.skillBinds[key] = binding.Clone()
	return binding.Clone(), true, nil
}

func promptBindingKey(agentID string, promptID string) string {
	return agentID + "\x00" + promptID
}

func toolBindingKey(agentID string, toolID string) string {
	return agentID + "\x00" + toolID
}

func skillBindingKey(agentID string, skillID string) string {
	return agentID + "\x00" + skillID
}
