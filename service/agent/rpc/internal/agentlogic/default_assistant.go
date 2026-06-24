package agentlogic

import (
	"context"

	"github.com/wujunhui99/agents_im/pkg/model"
)

const (
	DefaultAssistantIdentifier       = "agent_creator"
	DefaultAssistantAgentName        = DefaultAssistantIdentifier
	DefaultAssistantAgentDescription = "Default general AI assistant"
	DefaultAssistantPromptName       = "agent_creator_default_system_prompt"
	DefaultAssistantPromptVersion    = "v1"
	DefaultAssistantSystemPrompt     = "你是一个通用 AI 助手，回答应准确、简洁、友好。你可以帮助用户解释概念、比较方案、整理信息、生成文本和提供编程/产品建议。不要编造事实；不确定时说明不确定并给出可验证的下一步。当用户明确要求创建新的 Agent 时，可以使用 agent.create 工具创建账号、Agent 配置、系统提示词、允许的低风险工具绑定，并把新 Agent 加为该用户好友。"
	DefaultAssistantPythonToolName   = model.LocalToolHandlerPythonExecute
	DefaultAssistantAgentCreateName  = model.LocalToolHandlerAgentCreate
)

const defaultAssistantPythonToolInputSchema = `{
  "type": "object",
  "additionalProperties": false,
  "properties": {
    "code": {
      "type": "string",
      "description": "Python code to execute in the configured sandbox."
    },
    "timeout_seconds": {
      "type": "integer",
      "minimum": 1,
      "maximum": 30,
      "description": "Optional execution timeout in seconds."
    },
    "files": {
      "type": "array",
      "items": {
        "type": "string"
      },
      "description": "Optional read-only allowlisted file paths. Empty unless explicitly configured."
    }
  },
  "required": ["code"]
}`

const defaultAssistantPythonToolOutputSchema = `{
  "type": "object",
  "properties": {
    "stdout": {"type": "string"},
    "stderr": {"type": "string"},
    "result_json": {},
    "exit_code": {"type": "integer"},
    "timed_out": {"type": "boolean"},
    "output_truncated": {"type": "boolean"},
    "error": {
      "type": ["object", "null"],
      "properties": {
        "code": {"type": "string"},
        "message": {"type": "string"}
      }
    }
  }
}`

const defaultAssistantAgentCreateInputSchema = `{
  "type": "object",
  "additionalProperties": false,
  "properties": {
    "identifier": {
      "type": "string",
      "description": "Optional unique account identifier. If omitted the server allocates one."
    },
    "name": {
      "type": "string",
      "description": "Display name for the new Agent account and Agent profile."
    },
    "description": {
      "type": "string",
      "description": "Human-facing Agent purpose or job description."
    },
    "system_prompt": {
      "type": "string",
      "description": "Optional system prompt to bind as the Agent definition. If omitted, the service generates one from name and description."
    },
    "tool_names": {
      "type": "array",
      "items": {"type": "string"},
      "description": "Optional low-risk tool names to bind. High-risk write, Python, MCP/network, and agent.create tools are rejected by policy."
    }
  },
  "required": ["name", "description"]
}`

const defaultAssistantAgentCreateOutputSchema = `{
  "type": "object",
  "properties": {
    "agent_id": {"type": "string"},
    "account_id": {"type": "string"},
    "identifier": {"type": "string"},
    "name": {"type": "string"},
    "description": {"type": "string"},
    "prompt_id": {"type": "string"},
    "tool_names": {
      "type": "array",
      "items": {"type": "string"}
    },
    "friend_user_id": {"type": "string"}
  }
}`

// DefaultAssistantProvisioner 装配默认助手的 **agent 域** 部分（agent 行 + 默认提示词 + python/
// agent.create 工具及其绑定），#606 从 internal/logic.DefaultAssistantProvisioner 拆出。账号属 user 域、
// 好友属 friends 域，由 user-rpc 装配编排（经 agent-rpc.EnsureDefaultAssistant + friends-rpc）。
type DefaultAssistantProvisioner struct {
	agents   AgentStore
	registry RegistryStore
}

func NewDefaultAssistantProvisioner(agents AgentStore, registry RegistryStore) *DefaultAssistantProvisioner {
	return &DefaultAssistantProvisioner{agents: agents, registry: registry}
}

// DefaultAssistantResult 是 EnsureDefaultAssistant 的幂等结果（已存在则返回既有 ID）。
type DefaultAssistantResult struct {
	AgentID  string
	PromptID string
}

// EnsureDefaultAssistant 幂等确保给定 agent_creator 账号的 agent 行 + 提示词 + 工具绑定就绪。
// accountID 由 user-rpc 传入（账号已建并设好资料）。非事务、可重入。
func (p *DefaultAssistantProvisioner) EnsureDefaultAssistant(ctx context.Context, accountID string) (DefaultAssistantResult, error) {
	accountID, err := normalizeRequiredID(accountID, "account_id")
	if err != nil {
		return DefaultAssistantResult{}, err
	}
	agent, err := p.ensureAgent(ctx, accountID)
	if err != nil {
		return DefaultAssistantResult{}, err
	}
	prompt, err := p.ensurePrompt(ctx, accountID)
	if err != nil {
		return DefaultAssistantResult{}, err
	}
	if _, _, err := p.registry.BindPrompt(ctx, model.AgentPromptBinding{
		AgentID:   agent.AgentID,
		PromptID:  prompt.PromptID,
		CreatedBy: accountID,
	}); err != nil {
		return DefaultAssistantResult{}, err
	}
	tool, err := p.ensurePythonExecuteTool(ctx, accountID)
	if err != nil {
		return DefaultAssistantResult{}, err
	}
	if _, _, err := p.registry.BindTool(ctx, model.AgentToolBinding{
		AgentID:   agent.AgentID,
		ToolID:    tool.ToolID,
		CreatedBy: accountID,
	}); err != nil {
		return DefaultAssistantResult{}, err
	}
	createTool, err := p.ensureAgentCreateTool(ctx, accountID)
	if err != nil {
		return DefaultAssistantResult{}, err
	}
	if _, _, err := p.registry.BindTool(ctx, model.AgentToolBinding{
		AgentID:   agent.AgentID,
		ToolID:    createTool.ToolID,
		CreatedBy: accountID,
	}); err != nil {
		return DefaultAssistantResult{}, err
	}
	return DefaultAssistantResult{AgentID: agent.AgentID, PromptID: prompt.PromptID}, nil
}

func (p *DefaultAssistantProvisioner) ensureAgent(ctx context.Context, accountID string) (model.Agent, error) {
	agent, err := p.agents.GetAgentByAccountID(ctx, accountID)
	if err != nil {
		if !isAppNotFound(err) {
			return model.Agent{}, err
		}
		return p.agents.CreateAgent(ctx, model.Agent{
			AccountID:   accountID,
			IMUserID:    accountID,
			Name:        DefaultAssistantAgentName,
			Description: DefaultAssistantAgentDescription,
			Status:      model.AgentStatusActive,
			CreatedBy:   accountID,
		})
	}
	if agent.Name != DefaultAssistantAgentName || agent.Description != DefaultAssistantAgentDescription {
		name := DefaultAssistantAgentName
		description := DefaultAssistantAgentDescription
		updated, updateErr := p.agents.UpdateAgent(ctx, agent.AgentID, &name, &description)
		if updateErr != nil {
			return model.Agent{}, updateErr
		}
		agent = updated
	}
	if agent.Status != model.AgentStatusActive {
		updated, updateErr := p.agents.UpdateAgentStatus(ctx, agent.AgentID, model.AgentStatusActive)
		if updateErr != nil {
			return model.Agent{}, updateErr
		}
		agent = updated
	}
	return agent, nil
}

func (p *DefaultAssistantProvisioner) ensurePrompt(ctx context.Context, accountID string) (model.AgentPrompt, error) {
	prompt, err := p.registry.GetPromptByNameVersion(ctx, DefaultAssistantPromptName, DefaultAssistantPromptVersion)
	if err == nil {
		return prompt, nil
	}
	if !isAppNotFound(err) {
		return model.AgentPrompt{}, err
	}
	return p.registry.CreatePrompt(ctx, model.AgentPrompt{
		Name:                DefaultAssistantPromptName,
		Description:         "System prompt for the default agent_creator assistant",
		Content:             DefaultAssistantSystemPrompt,
		VariablesSchemaJSON: "{}",
		Version:             DefaultAssistantPromptVersion,
		Status:              model.AgentPromptStatusActive,
		CreatedBy:           accountID,
	})
}

func (p *DefaultAssistantProvisioner) ensurePythonExecuteTool(ctx context.Context, accountID string) (model.AgentTool, error) {
	return p.registry.UpsertToolByName(ctx, model.AgentTool{
		Name:             DefaultAssistantPythonToolName,
		Description:      "Execute bounded Python code through the configured sandbox executor.",
		ToolType:         model.AgentToolTypeLocal,
		LocalHandlerKey:  model.LocalToolHandlerPythonExecute,
		InputSchemaJSON:  defaultAssistantPythonToolInputSchema,
		OutputSchemaJSON: defaultAssistantPythonToolOutputSchema,
		PermissionLevel:  "restricted",
		Status:           model.AgentToolStatusActive,
		AdminConfigured:  true,
		CreatedBy:        accountID,
	})
}

func (p *DefaultAssistantProvisioner) ensureAgentCreateTool(ctx context.Context, accountID string) (model.AgentTool, error) {
	return p.registry.UpsertToolByName(ctx, model.AgentTool{
		Name:             DefaultAssistantAgentCreateName,
		Description:      "Create a new Agent through the server-side agent assembly workflow.",
		ToolType:         model.AgentToolTypeLocal,
		LocalHandlerKey:  model.LocalToolHandlerAgentCreate,
		InputSchemaJSON:  defaultAssistantAgentCreateInputSchema,
		OutputSchemaJSON: defaultAssistantAgentCreateOutputSchema,
		PermissionLevel:  "restricted",
		Status:           model.AgentToolStatusActive,
		AdminConfigured:  true,
		CreatedBy:        accountID,
	})
}
