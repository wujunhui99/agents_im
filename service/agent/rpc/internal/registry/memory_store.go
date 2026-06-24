package registry

import (
	"strconv"
	"sync"

	"context"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/pkg/model"
)

// MemoryStore 是注册表的内存实现，仅供单测 fixture 使用（替代旧 internal/repository.MemoryAgentRegistryRepository，
// #606 退役 internal 后）。实现 Reader + agentlogic.RegistryStore 全量读写；ID 由内部计数器分配（十进制串）。
// 生产路径用 Store（goctl model），绝不用本类型。
type MemoryStore struct {
	mu             sync.Mutex
	seq            int64
	prompts        map[string]model.AgentPrompt
	tools          map[string]model.AgentTool
	toolsByName    map[string]string
	skills         map[string]model.AgentSkill
	mcpServers     map[string]model.AgentMCPServer
	promptBindings map[string][]model.AgentPromptBinding
	toolBindings   map[string][]model.AgentToolBinding
	skillBindings  map[string][]model.AgentSkillBinding
}

var _ Reader = (*MemoryStore)(nil)

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		prompts:        map[string]model.AgentPrompt{},
		tools:          map[string]model.AgentTool{},
		toolsByName:    map[string]string{},
		skills:         map[string]model.AgentSkill{},
		mcpServers:     map[string]model.AgentMCPServer{},
		promptBindings: map[string][]model.AgentPromptBinding{},
		toolBindings:   map[string][]model.AgentToolBinding{},
		skillBindings:  map[string][]model.AgentSkillBinding{},
	}
}

func (s *MemoryStore) nextID() string {
	s.seq++
	return strconv.FormatInt(s.seq, 10)
}

// idOr 沿用旧 internal 内存 fixture 语义：调用方显式给定 ID 则用之（便于测试按已知 ID 绑定），
// 否则分配内部计数器 ID。
func (s *MemoryStore) idOr(provided string) string {
	if provided != "" {
		return provided
	}
	return s.nextID()
}

func (s *MemoryStore) CreatePrompt(_ context.Context, prompt model.AgentPrompt) (model.AgentPrompt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	prompt.PromptID = s.idOr(prompt.PromptID)
	s.prompts[prompt.PromptID] = prompt
	return prompt, nil
}

func (s *MemoryStore) GetPrompt(_ context.Context, promptID string) (model.AgentPrompt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	prompt, ok := s.prompts[promptID]
	if !ok {
		return model.AgentPrompt{}, apperror.NotFound("prompt not found")
	}
	return prompt, nil
}

func (s *MemoryStore) GetPromptByNameVersion(_ context.Context, name string, version string) (model.AgentPrompt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, prompt := range s.prompts {
		if prompt.Name == name && prompt.Version == version {
			return prompt, nil
		}
	}
	return model.AgentPrompt{}, apperror.NotFound("prompt not found")
}

func (s *MemoryStore) BindPrompt(_ context.Context, binding model.AgentPromptBinding) (model.AgentPromptBinding, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, existing := range s.promptBindings[binding.AgentID] {
		if existing.PromptID == binding.PromptID {
			return existing, false, nil
		}
	}
	s.promptBindings[binding.AgentID] = append(s.promptBindings[binding.AgentID], binding)
	return binding, true, nil
}

func (s *MemoryStore) ListPromptBindings(_ context.Context, agentID string) ([]model.AgentPromptBinding, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]model.AgentPromptBinding(nil), s.promptBindings[agentID]...), nil
}

func (s *MemoryStore) ReplacePromptBindings(_ context.Context, agentID string, promptIDs []string, createdBy string) ([]model.AgentPromptBinding, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	bindings := make([]model.AgentPromptBinding, 0, len(promptIDs))
	seen := map[string]struct{}{}
	for _, id := range promptIDs {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		bindings = append(bindings, model.AgentPromptBinding{AgentID: agentID, PromptID: id, CreatedBy: createdBy})
	}
	s.promptBindings[agentID] = bindings
	return append([]model.AgentPromptBinding(nil), bindings...), nil
}

func (s *MemoryStore) CreateMCPServer(_ context.Context, server model.AgentMCPServer) (model.AgentMCPServer, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	server.ServerID = s.idOr(server.ServerID)
	s.mcpServers[server.ServerID] = server
	return server, nil
}

func (s *MemoryStore) GetMCPServer(_ context.Context, serverID string) (model.AgentMCPServer, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	server, ok := s.mcpServers[serverID]
	if !ok {
		return model.AgentMCPServer{}, apperror.NotFound("mcp server not found")
	}
	return server, nil
}

func (s *MemoryStore) RegisterTool(_ context.Context, tool model.AgentTool) (model.AgentTool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.toolsByName[tool.Name]; ok {
		return model.AgentTool{}, apperror.AlreadyExists("tool already exists")
	}
	tool.ToolID = s.idOr(tool.ToolID)
	s.tools[tool.ToolID] = tool
	s.toolsByName[tool.Name] = tool.ToolID
	return tool, nil
}

func (s *MemoryStore) UpsertToolByName(_ context.Context, tool model.AgentTool) (model.AgentTool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if id, ok := s.toolsByName[tool.Name]; ok {
		tool.ToolID = id
		s.tools[id] = tool
		return tool, nil
	}
	tool.ToolID = s.nextID()
	s.tools[tool.ToolID] = tool
	s.toolsByName[tool.Name] = tool.ToolID
	return tool, nil
}

func (s *MemoryStore) GetTool(_ context.Context, toolID string) (model.AgentTool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	tool, ok := s.tools[toolID]
	if !ok {
		return model.AgentTool{}, apperror.NotFound("tool not found")
	}
	return tool, nil
}

func (s *MemoryStore) GetToolByName(_ context.Context, name string) (model.AgentTool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id, ok := s.toolsByName[name]
	if !ok {
		return model.AgentTool{}, apperror.NotFound("tool not found")
	}
	return s.tools[id], nil
}

func (s *MemoryStore) ListActiveTools(_ context.Context) ([]model.AgentTool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	tools := make([]model.AgentTool, 0, len(s.tools))
	for _, tool := range s.tools {
		if tool.Status == model.AgentToolStatusActive {
			tools = append(tools, tool)
		}
	}
	return tools, nil
}

func (s *MemoryStore) BindTool(_ context.Context, binding model.AgentToolBinding) (model.AgentToolBinding, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, existing := range s.toolBindings[binding.AgentID] {
		if existing.ToolID == binding.ToolID {
			return existing, false, nil
		}
	}
	s.toolBindings[binding.AgentID] = append(s.toolBindings[binding.AgentID], binding)
	return binding, true, nil
}

func (s *MemoryStore) GetToolBinding(_ context.Context, agentID string, toolID string) (model.AgentToolBinding, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, existing := range s.toolBindings[agentID] {
		if existing.ToolID == toolID {
			return existing, nil
		}
	}
	return model.AgentToolBinding{}, apperror.NotFound("tool binding not found")
}

func (s *MemoryStore) ListToolBindings(_ context.Context, agentID string) ([]model.AgentToolBinding, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]model.AgentToolBinding(nil), s.toolBindings[agentID]...), nil
}

func (s *MemoryStore) ReplaceToolBindings(_ context.Context, agentID string, toolIDs []string, createdBy string) ([]model.AgentToolBinding, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	bindings := make([]model.AgentToolBinding, 0, len(toolIDs))
	seen := map[string]struct{}{}
	for _, id := range toolIDs {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		bindings = append(bindings, model.AgentToolBinding{AgentID: agentID, ToolID: id, CreatedBy: createdBy})
	}
	s.toolBindings[agentID] = bindings
	return append([]model.AgentToolBinding(nil), bindings...), nil
}

func (s *MemoryStore) RegisterSkill(_ context.Context, skill model.AgentSkill) (model.AgentSkill, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	skill.SkillID = s.idOr(skill.SkillID)
	s.skills[skill.SkillID] = skill
	return skill, nil
}

func (s *MemoryStore) GetSkill(_ context.Context, skillID string) (model.AgentSkill, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	skill, ok := s.skills[skillID]
	if !ok {
		return model.AgentSkill{}, apperror.NotFound("skill not found")
	}
	return skill, nil
}

func (s *MemoryStore) BindSkill(_ context.Context, binding model.AgentSkillBinding) (model.AgentSkillBinding, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, existing := range s.skillBindings[binding.AgentID] {
		if existing.SkillID == binding.SkillID {
			return existing, false, nil
		}
	}
	s.skillBindings[binding.AgentID] = append(s.skillBindings[binding.AgentID], binding)
	return binding, true, nil
}
