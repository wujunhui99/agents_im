package agent

import (
	"github.com/wujunhui99/agents_im/pkg/apperror"
	business "github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/service/agent/api/internal/types"
)

func optionalAgentString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func agentResp(agent business.AgentInfo) *types.AgentResp {
	return &types.AgentResp{
		Code:    string(apperror.CodeOK),
		Message: "ok",
		Data:    agentType(agent),
	}
}

func agentDefinitionResp(definition business.AgentDefinition) *types.AgentDefinitionResp {
	return &types.AgentDefinitionResp{
		Code:    string(apperror.CodeOK),
		Message: "ok",
		Data: types.AgentDefinitionData{
			Agent:        agentType(definition.Agent),
			SystemPrompt: agentDefinitionPrompt(definition.SystemPrompt),
			Tools:        agentDefinitionTools(definition.Tools),
		},
	}
}

func agentType(agent business.AgentInfo) types.Agent {
	return types.Agent{
		AgentID:     agent.AgentID,
		IMUserID:    agent.IMUserID,
		Name:        agent.Name,
		Description: agent.Description,
		Status:      agent.Status,
		CreatedBy:   agent.CreatedBy,
		CreatedAt:   agent.CreatedAt,
		UpdatedAt:   agent.UpdatedAt,
	}
}

func agentDefinitionPrompt(prompt business.AgentPromptDefinition) types.AgentDefinitionPrompt {
	return types.AgentDefinitionPrompt{
		PromptID:            prompt.PromptID,
		Name:                prompt.Name,
		Description:         prompt.Description,
		Content:             prompt.Content,
		VariablesSchemaJSON: prompt.VariablesSchemaJSON,
		Version:             prompt.Version,
		Status:              prompt.Status,
		CreatedBy:           prompt.CreatedBy,
		CreatedAt:           prompt.CreatedAt,
		UpdatedAt:           prompt.UpdatedAt,
	}
}

func agentDefinitionTools(tools []business.AgentToolDefinition) []types.AgentDefinitionTool {
	result := make([]types.AgentDefinitionTool, 0, len(tools))
	for _, tool := range tools {
		result = append(result, types.AgentDefinitionTool{
			ToolID:           tool.ToolID,
			Name:             tool.Name,
			Description:      tool.Description,
			ToolType:         tool.ToolType,
			MCPServerID:      tool.MCPServerID,
			MCPToolName:      tool.MCPToolName,
			LocalHandlerKey:  tool.LocalHandlerKey,
			BuiltinKey:       tool.BuiltinKey,
			InputSchemaJSON:  tool.InputSchemaJSON,
			OutputSchemaJSON: tool.OutputSchemaJSON,
			PermissionLevel:  tool.PermissionLevel,
			Status:           tool.Status,
			AdminConfigured:  tool.AdminConfigured,
		})
	}
	return result
}
