package agent

import (
	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/service/agent/api/internal/types"
	agentpb "github.com/wujunhui99/agents_im/service/agent/rpc/agent"
)

// convert.go 把 agent-rpc 的 proto 响应映射成 agent-api 的 BFF 视图类型（#606：纯 BFF over gRPC）。

func agentRespFromPB(entity *agentpb.AgentEntity) *types.AgentResp {
	return &types.AgentResp{
		Code:    string(apperror.CodeOK),
		Message: "ok",
		Data:    agentTypeFromPB(entity),
	}
}

func agentTypeFromPB(entity *agentpb.AgentEntity) types.Agent {
	if entity == nil {
		return types.Agent{}
	}
	return types.Agent{
		AgentID:     entity.GetAgentId(),
		IMUserID:    entity.GetImUserId(),
		Name:        entity.GetName(),
		Description: entity.GetDescription(),
		Status:      entity.GetStatus(),
		CreatedBy:   entity.GetCreatedBy(),
		CreatedAt:   entity.GetCreatedAt(),
		UpdatedAt:   entity.GetUpdatedAt(),
	}
}

func agentDefinitionRespFromPB(definition *agentpb.AgentDefinition) *types.AgentDefinitionResp {
	return &types.AgentDefinitionResp{
		Code:    string(apperror.CodeOK),
		Message: "ok",
		Data: types.AgentDefinitionData{
			Agent:        agentTypeFromPB(definition.GetAgent()),
			SystemPrompt: agentDefinitionPromptFromPB(definition.GetSystemPrompt()),
			Tools:        agentDefinitionToolsFromPB(definition.GetTools()),
		},
	}
}

func agentDefinitionPromptFromPB(prompt *agentpb.AgentPromptDefinition) types.AgentDefinitionPrompt {
	if prompt == nil {
		return types.AgentDefinitionPrompt{}
	}
	return types.AgentDefinitionPrompt{
		PromptID:            prompt.GetPromptId(),
		Name:                prompt.GetName(),
		Description:         prompt.GetDescription(),
		Content:             prompt.GetContent(),
		VariablesSchemaJSON: prompt.GetVariablesSchemaJson(),
		Version:             prompt.GetVersion(),
		Status:              prompt.GetStatus(),
		CreatedBy:           prompt.GetCreatedBy(),
		CreatedAt:           prompt.GetCreatedAt(),
		UpdatedAt:           prompt.GetUpdatedAt(),
	}
}

func agentDefinitionToolsFromPB(tools []*agentpb.AgentToolDefinition) []types.AgentDefinitionTool {
	result := make([]types.AgentDefinitionTool, 0, len(tools))
	for _, tool := range tools {
		result = append(result, types.AgentDefinitionTool{
			ToolID:           tool.GetToolId(),
			Name:             tool.GetName(),
			Description:      tool.GetDescription(),
			ToolType:         tool.GetToolType(),
			MCPServerID:      tool.GetMcpServerId(),
			MCPToolName:      tool.GetMcpToolName(),
			LocalHandlerKey:  tool.GetLocalHandlerKey(),
			BuiltinKey:       tool.GetBuiltinKey(),
			InputSchemaJSON:  tool.GetInputSchemaJson(),
			OutputSchemaJSON: tool.GetOutputSchemaJson(),
			PermissionLevel:  tool.GetPermissionLevel(),
			Status:           tool.GetStatus(),
			AdminConfigured:  tool.GetAdminConfigured(),
		})
	}
	return result
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
