package registry

import (
	"database/sql"

	"context"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/pkg/model"
	rpcmodel "github.com/wujunhui99/agents_im/service/agent/rpc/internal/model"
)

// write.go 给 registry.Store 补 #606 注册表写路径（AgentRegistryLogic / AgentAssemblyLogic /
// DefaultAssistant 装配所需），全部走 agent 自有 goctl model（无 internal/repository）。对外 string ID，
// 内部 string↔int64 转换；唯一冲突→AlreadyExists、外键冲突→NotFound、check 冲突→InvalidArgument。

// ---- prompts ----

func (s *Store) CreatePrompt(ctx context.Context, prompt model.AgentPrompt) (model.AgentPrompt, error) {
	row, err := s.prompts.InsertReturning(ctx, &rpcmodel.AgentPrompts{
		Name:                prompt.Name,
		Description:         prompt.Description,
		Content:             prompt.Content,
		VariablesSchemaJson: prompt.VariablesSchemaJSON,
		Version:             prompt.Version,
		Status:              string(prompt.Status),
		CreatedBy:           prompt.CreatedBy,
	})
	if err != nil {
		return model.AgentPrompt{}, mapWriteError(err, "prompt already exists")
	}
	return promptFromModel(row), nil
}

func (s *Store) GetPromptByNameVersion(ctx context.Context, name string, version string) (model.AgentPrompt, error) {
	row, err := s.prompts.FindOneByNameVersion(ctx, name, version)
	if err != nil {
		return model.AgentPrompt{}, mapNotFound(err, "prompt not found")
	}
	return promptFromModel(row), nil
}

func (s *Store) BindPrompt(ctx context.Context, binding model.AgentPromptBinding) (model.AgentPromptBinding, bool, error) {
	agentID, aok := parseID(binding.AgentID)
	promptID, pok := parseID(binding.PromptID)
	if !aok || !pok {
		return model.AgentPromptBinding{}, false, apperror.NotFound("prompt not found")
	}
	row, created, err := s.promptBindings.BindOne(ctx, agentID, promptID, binding.CreatedBy)
	if err != nil {
		if rpcmodel.IsForeignKeyViolation(err) {
			return model.AgentPromptBinding{}, false, apperror.NotFound("prompt not found")
		}
		return model.AgentPromptBinding{}, false, err
	}
	return promptBindingFromModel(row), created, nil
}

func (s *Store) ReplacePromptBindings(ctx context.Context, agentID string, promptIDs []string, createdBy string) ([]model.AgentPromptBinding, error) {
	aid, ok := parseID(agentID)
	if !ok {
		return nil, apperror.NotFound("agent not found")
	}
	ids, err := parseIDs(promptIDs, "prompt not found")
	if err != nil {
		return nil, err
	}
	rows, err := s.promptBindings.ReplaceForAgent(ctx, aid, ids, createdBy)
	if err != nil {
		return nil, mapBindingWriteError(err, "prompt not found")
	}
	bindings := make([]model.AgentPromptBinding, 0, len(rows))
	for _, row := range rows {
		bindings = append(bindings, promptBindingFromModel(row))
	}
	return bindings, nil
}

// ---- mcp servers ----

func (s *Store) CreateMCPServer(ctx context.Context, server model.AgentMCPServer) (model.AgentMCPServer, error) {
	row, err := s.mcpServers.InsertReturning(ctx, &rpcmodel.McpServers{
		Name:             server.Name,
		Transport:        string(server.Transport),
		Url:              server.URL,
		ConfigJson:       server.ConfigJSON,
		HeadersSecretRef: server.HeadersSecretRef,
		TimeoutSeconds:   int64(server.TimeoutSeconds),
		Status:           string(server.Status),
		AdminConfigured:  server.AdminConfigured,
		CreatedBy:        server.CreatedBy,
	})
	if err != nil {
		return model.AgentMCPServer{}, mapWriteError(err, "mcp server already exists")
	}
	return mcpServerFromModel(row), nil
}

// ---- tools ----

func (s *Store) RegisterTool(ctx context.Context, tool model.AgentTool) (model.AgentTool, error) {
	row, err := s.tools.InsertReturning(ctx, toolToModel(tool))
	if err != nil {
		if rpcmodel.IsForeignKeyViolation(err) {
			return model.AgentTool{}, apperror.NotFound("mcp server not found")
		}
		return model.AgentTool{}, mapWriteError(err, "tool already exists")
	}
	return toolFromModel(row), nil
}

func (s *Store) UpsertToolByName(ctx context.Context, tool model.AgentTool) (model.AgentTool, error) {
	row, err := s.tools.UpsertByName(ctx, toolToModel(tool))
	if err != nil {
		if rpcmodel.IsForeignKeyViolation(err) {
			return model.AgentTool{}, apperror.NotFound("mcp server not found")
		}
		return model.AgentTool{}, mapWriteError(err, "tool already exists")
	}
	return toolFromModel(row), nil
}

func (s *Store) GetToolByName(ctx context.Context, name string) (model.AgentTool, error) {
	row, err := s.tools.FindOneByName(ctx, name)
	if err != nil {
		return model.AgentTool{}, mapNotFound(err, "tool not found")
	}
	return toolFromModel(row), nil
}

func (s *Store) ListActiveTools(ctx context.Context) ([]model.AgentTool, error) {
	rows, err := s.tools.ListActive(ctx)
	if err != nil {
		return nil, err
	}
	tools := make([]model.AgentTool, 0, len(rows))
	for _, row := range rows {
		tools = append(tools, toolFromModel(row))
	}
	return tools, nil
}

func (s *Store) BindTool(ctx context.Context, binding model.AgentToolBinding) (model.AgentToolBinding, bool, error) {
	agentID, aok := parseID(binding.AgentID)
	toolID, tok := parseID(binding.ToolID)
	if !aok || !tok {
		return model.AgentToolBinding{}, false, apperror.NotFound("tool not found")
	}
	row, created, err := s.toolBindings.BindOne(ctx, agentID, toolID, binding.CreatedBy)
	if err != nil {
		if rpcmodel.IsForeignKeyViolation(err) {
			return model.AgentToolBinding{}, false, apperror.NotFound("tool not found")
		}
		return model.AgentToolBinding{}, false, err
	}
	return toolBindingFromModel(row), created, nil
}

func (s *Store) ReplaceToolBindings(ctx context.Context, agentID string, toolIDs []string, createdBy string) ([]model.AgentToolBinding, error) {
	aid, ok := parseID(agentID)
	if !ok {
		return nil, apperror.NotFound("agent not found")
	}
	ids, err := parseIDs(toolIDs, "tool not found")
	if err != nil {
		return nil, err
	}
	rows, err := s.toolBindings.ReplaceForAgent(ctx, aid, ids, createdBy)
	if err != nil {
		return nil, mapBindingWriteError(err, "tool not found")
	}
	bindings := make([]model.AgentToolBinding, 0, len(rows))
	for _, row := range rows {
		bindings = append(bindings, toolBindingFromModel(row))
	}
	return bindings, nil
}

// ---- skills ----

func (s *Store) RegisterSkill(ctx context.Context, skill model.AgentSkill) (model.AgentSkill, error) {
	row, err := s.skills.InsertReturning(ctx, &rpcmodel.AgentSkills{
		Name:        skill.Name,
		Description: skill.Description,
		Version:     skill.Version,
		ObjectKey:   skill.ObjectKey,
		Sha256:      skill.SHA256,
		ContentType: skill.ContentType,
		SizeBytes:   skill.SizeBytes,
		Status:      string(skill.Status),
		CreatedBy:   skill.CreatedBy,
	})
	if err != nil {
		return model.AgentSkill{}, mapWriteError(err, "skill already exists")
	}
	return skillFromModel(row), nil
}

func (s *Store) GetSkill(ctx context.Context, skillID string) (model.AgentSkill, error) {
	id, ok := parseID(skillID)
	if !ok {
		return model.AgentSkill{}, apperror.NotFound("skill not found")
	}
	row, err := s.skills.FindOne(ctx, id)
	if err != nil {
		return model.AgentSkill{}, mapNotFound(err, "skill not found")
	}
	return skillFromModel(row), nil
}

func (s *Store) BindSkill(ctx context.Context, binding model.AgentSkillBinding) (model.AgentSkillBinding, bool, error) {
	agentID, aok := parseID(binding.AgentID)
	skillID, sok := parseID(binding.SkillID)
	if !aok || !sok {
		return model.AgentSkillBinding{}, false, apperror.NotFound("skill not found")
	}
	row, created, err := s.skillBindings.BindOne(ctx, agentID, skillID, binding.CreatedBy)
	if err != nil {
		if rpcmodel.IsForeignKeyViolation(err) {
			return model.AgentSkillBinding{}, false, apperror.NotFound("skill not found")
		}
		return model.AgentSkillBinding{}, false, err
	}
	return model.AgentSkillBinding{
		AgentID:   formatID(row.AgentId),
		SkillID:   formatID(row.SkillId),
		CreatedBy: row.CreatedBy,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}, created, nil
}

// ---- helpers ----

func toolToModel(tool model.AgentTool) *rpcmodel.AgentTools {
	var mcpServerID sql.NullInt64
	if id, ok := parseID(tool.MCPServerID); ok {
		mcpServerID = sql.NullInt64{Int64: id, Valid: true}
	}
	return &rpcmodel.AgentTools{
		Name:             tool.Name,
		Description:      tool.Description,
		ToolType:         string(tool.ToolType),
		McpServerId:      mcpServerID,
		McpToolName:      tool.MCPToolName,
		LocalHandlerKey:  tool.LocalHandlerKey,
		BuiltinKey:       tool.BuiltinKey,
		InputSchemaJson:  tool.InputSchemaJSON,
		OutputSchemaJson: tool.OutputSchemaJSON,
		PermissionLevel:  tool.PermissionLevel,
		Status:           string(tool.Status),
		AdminConfigured:  tool.AdminConfigured,
		CreatedBy:        tool.CreatedBy,
	}
}

func skillFromModel(row *rpcmodel.AgentSkills) model.AgentSkill {
	return model.AgentSkill{
		SkillID:     formatID(row.SkillId),
		Name:        row.Name,
		Description: row.Description,
		Version:     row.Version,
		ObjectKey:   row.ObjectKey,
		SHA256:      row.Sha256,
		ContentType: row.ContentType,
		SizeBytes:   row.SizeBytes,
		Status:      model.AgentSkillStatus(row.Status),
		CreatedBy:   row.CreatedBy,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}
}

func promptBindingFromModel(row *rpcmodel.AgentPromptBindings) model.AgentPromptBinding {
	return model.AgentPromptBinding{
		AgentID:   formatID(row.AgentId),
		PromptID:  formatID(row.PromptId),
		CreatedBy: row.CreatedBy,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}
}

func toolBindingFromModel(row *rpcmodel.AgentToolBindings) model.AgentToolBinding {
	return model.AgentToolBinding{
		AgentID:   formatID(row.AgentId),
		ToolID:    formatID(row.ToolId),
		CreatedBy: row.CreatedBy,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}
}

func parseIDs(values []string, notFoundMessage string) ([]int64, error) {
	ids := make([]int64, 0, len(values))
	for _, value := range values {
		id, ok := parseID(value)
		if !ok {
			return nil, apperror.NotFound(notFoundMessage)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func mapWriteError(err error, duplicateMessage string) error {
	if rpcmodel.IsUniqueViolation(err) {
		return apperror.AlreadyExists(duplicateMessage)
	}
	if rpcmodel.IsCheckViolation(err) {
		return apperror.InvalidArgument("invalid value")
	}
	return err
}

func mapBindingWriteError(err error, notFoundMessage string) error {
	if rpcmodel.IsForeignKeyViolation(err) {
		return apperror.NotFound(notFoundMessage)
	}
	if rpcmodel.IsCheckViolation(err) {
		return apperror.InvalidArgument("invalid binding")
	}
	return err
}
