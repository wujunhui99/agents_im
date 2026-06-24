// agent_definition_logic.go 是 agent 定义（系统提示词 + 工具绑定）+ 默认助手装配的 gRPC 处理器
// （#606）：薄封装 svcCtx.AgentAssembly / AgentProvisioner（agent 自有 goctl + 跨域端口）。
package logic

import (
	"context"

	"github.com/wujunhui99/agents_im/pkg/rpcerror"
	"github.com/wujunhui99/agents_im/service/agent/rpc/agent"
	"github.com/wujunhui99/agents_im/service/agent/rpc/internal/agentlogic"
	"github.com/wujunhui99/agents_im/service/agent/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetAgentDefinitionLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetAgentDefinitionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetAgentDefinitionLogic {
	return &GetAgentDefinitionLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *GetAgentDefinitionLogic) GetAgentDefinition(in *agent.GetAgentDefinitionRequest) (*agent.AgentDefinitionResponse, error) {
	def, err := l.svcCtx.AgentAssembly.GetAgentDefinition(l.ctx, agentlogic.AgentDefinitionRequest{
		AgentID:     in.GetAgentId(),
		RequestedBy: in.GetRequestedBy(),
	})
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	return &agent.AgentDefinitionResponse{Definition: agentDefinitionToPB(def)}, nil
}

type UpdateAgentDefinitionLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateAgentDefinitionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateAgentDefinitionLogic {
	return &UpdateAgentDefinitionLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *UpdateAgentDefinitionLogic) UpdateAgentDefinition(in *agent.UpdateAgentDefinitionRequest) (*agent.AgentDefinitionResponse, error) {
	def, err := l.svcCtx.AgentAssembly.UpdateAgentDefinition(l.ctx, agentlogic.UpdateAgentDefinitionRequest{
		AgentID:      in.GetAgentId(),
		SystemPrompt: in.GetSystemPrompt(),
		ToolNames:    in.GetToolNames(),
		UpdatedBy:    in.GetUpdatedBy(),
	})
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	return &agent.AgentDefinitionResponse{Definition: agentDefinitionToPB(def)}, nil
}

type EnsureDefaultAssistantLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewEnsureDefaultAssistantLogic(ctx context.Context, svcCtx *svc.ServiceContext) *EnsureDefaultAssistantLogic {
	return &EnsureDefaultAssistantLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *EnsureDefaultAssistantLogic) EnsureDefaultAssistant(in *agent.EnsureDefaultAssistantRequest) (*agent.EnsureDefaultAssistantResponse, error) {
	result, err := l.svcCtx.AgentProvisioner.EnsureDefaultAssistant(l.ctx, in.GetAccountId())
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	return &agent.EnsureDefaultAssistantResponse{AgentId: result.AgentID, PromptId: result.PromptID}, nil
}

func agentDefinitionToPB(def agentlogic.AgentDefinition) *agent.AgentDefinition {
	tools := make([]*agent.AgentToolDefinition, 0, len(def.Tools))
	for _, tool := range def.Tools {
		tools = append(tools, &agent.AgentToolDefinition{
			ToolId:           tool.ToolID,
			Name:             tool.Name,
			Description:      tool.Description,
			ToolType:         tool.ToolType,
			McpServerId:      tool.MCPServerID,
			McpToolName:      tool.MCPToolName,
			LocalHandlerKey:  tool.LocalHandlerKey,
			BuiltinKey:       tool.BuiltinKey,
			InputSchemaJson:  tool.InputSchemaJSON,
			OutputSchemaJson: tool.OutputSchemaJSON,
			PermissionLevel:  tool.PermissionLevel,
			Status:           tool.Status,
			AdminConfigured:  tool.AdminConfigured,
		})
	}
	return &agent.AgentDefinition{
		Agent: agentInfoToPB(def.Agent),
		SystemPrompt: &agent.AgentPromptDefinition{
			PromptId:            def.SystemPrompt.PromptID,
			Name:                def.SystemPrompt.Name,
			Description:         def.SystemPrompt.Description,
			Content:             def.SystemPrompt.Content,
			VariablesSchemaJson: def.SystemPrompt.VariablesSchemaJSON,
			Version:             def.SystemPrompt.Version,
			Status:              def.SystemPrompt.Status,
			CreatedBy:           def.SystemPrompt.CreatedBy,
			CreatedAt:           def.SystemPrompt.CreatedAt,
			UpdatedAt:           def.SystemPrompt.UpdatedAt,
		},
		Tools: tools,
	}
}
