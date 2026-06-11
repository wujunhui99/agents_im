package logic

import (
	"context"

	"github.com/wujunhui99/agents_im/common/share/model"
	"github.com/wujunhui99/agents_im/internal/repository"
	"github.com/wujunhui99/agents_im/pkg/apperror"
)

const (
	DefaultAssistantIdentifier       = "agent_creator"
	DefaultAssistantLegacyIdentifier = "agent_father"
	DefaultAssistantDisplayName      = "AI 助手"
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

type DefaultAssistantProvisioner struct {
	accounts    repository.AccountRepository
	friendships repository.FriendshipRepository
	agents      repository.AgentRepository
	registry    repository.AgentRegistryRepository
}

type DefaultAssistantBackfillResult struct {
	AssistantAccountID string
	AgentID            string
	PromptID           string
	HumanUsersScanned  int
}

func NewDefaultAssistantProvisioner(repo repository.Repository, agents repository.AgentRepository, registry repository.AgentRegistryRepository) *DefaultAssistantProvisioner {
	return &DefaultAssistantProvisioner{
		accounts:    repo,
		friendships: repo,
		agents:      agents,
		registry:    registry,
	}
}

func (p *DefaultAssistantProvisioner) Backfill(ctx context.Context) (DefaultAssistantBackfillResult, error) {
	if err := p.ensureConfigured(); err != nil {
		return DefaultAssistantBackfillResult{}, err
	}

	assistant, agent, prompt, err := p.ensureDefaultAssistant(ctx)
	if err != nil {
		return DefaultAssistantBackfillResult{}, err
	}
	humans, err := p.accounts.ListByAccountType(ctx, model.AccountTypeUser)
	if err != nil {
		return DefaultAssistantBackfillResult{}, err
	}
	for _, human := range humans {
		if human.AccountID == assistant.AccountID {
			continue
		}
		if err := p.friendships.EnsureAcceptedFriendship(ctx, human.AccountID, assistant.AccountID); err != nil {
			return DefaultAssistantBackfillResult{}, err
		}
	}

	return DefaultAssistantBackfillResult{
		AssistantAccountID: assistant.AccountID,
		AgentID:            agent.AgentID,
		PromptID:           prompt.PromptID,
		HumanUsersScanned:  len(humans),
	}, nil
}

func (p *DefaultAssistantProvisioner) EnsureForUser(ctx context.Context, accountID string) error {
	if err := p.ensureConfigured(); err != nil {
		return err
	}
	account, err := p.accounts.GetByID(ctx, accountID)
	if err != nil {
		return err
	}
	// 测试账户（admin 后台创建）与 user 一致开通默认助手，便于测试 AI 链路。
	if account.AccountType != model.AccountTypeUser && account.AccountType != model.AccountTypeTest {
		return nil
	}
	assistant, _, _, err := p.ensureDefaultAssistant(ctx)
	if err != nil {
		return err
	}
	if account.AccountID == assistant.AccountID {
		return nil
	}
	return p.friendships.EnsureAcceptedFriendship(ctx, account.AccountID, assistant.AccountID)
}

func (p *DefaultAssistantProvisioner) ensureConfigured() error {
	if p == nil || p.accounts == nil || p.friendships == nil || p.agents == nil || p.registry == nil {
		return apperror.Internal("default assistant provisioner is not configured")
	}
	return nil
}

func (p *DefaultAssistantProvisioner) ensureDefaultAssistant(ctx context.Context) (model.User, model.Agent, model.AgentPrompt, error) {
	account, err := p.ensureDefaultAssistantAccount(ctx)
	if err != nil {
		return model.User{}, model.Agent{}, model.AgentPrompt{}, err
	}
	agent, err := p.ensureDefaultAssistantAgent(ctx, account)
	if err != nil {
		return model.User{}, model.Agent{}, model.AgentPrompt{}, err
	}
	prompt, err := p.ensureDefaultAssistantPrompt(ctx, account)
	if err != nil {
		return model.User{}, model.Agent{}, model.AgentPrompt{}, err
	}
	if _, _, err := p.registry.BindPrompt(ctx, model.AgentPromptBinding{
		AgentID:   agent.AgentID,
		PromptID:  prompt.PromptID,
		CreatedBy: account.AccountID,
	}); err != nil {
		return model.User{}, model.Agent{}, model.AgentPrompt{}, err
	}
	tool, err := p.ensurePythonExecuteTool(ctx, account)
	if err != nil {
		return model.User{}, model.Agent{}, model.AgentPrompt{}, err
	}
	if _, _, err := p.registry.BindTool(ctx, model.AgentToolBinding{
		AgentID:   agent.AgentID,
		ToolID:    tool.ToolID,
		CreatedBy: account.AccountID,
	}); err != nil {
		return model.User{}, model.Agent{}, model.AgentPrompt{}, err
	}
	createTool, err := p.ensureAgentCreateTool(ctx, account)
	if err != nil {
		return model.User{}, model.Agent{}, model.AgentPrompt{}, err
	}
	if _, _, err := p.registry.BindTool(ctx, model.AgentToolBinding{
		AgentID:   agent.AgentID,
		ToolID:    createTool.ToolID,
		CreatedBy: account.AccountID,
	}); err != nil {
		return model.User{}, model.Agent{}, model.AgentPrompt{}, err
	}
	return account, agent, prompt, nil
}

func (p *DefaultAssistantProvisioner) ensureDefaultAssistantAccount(ctx context.Context) (model.User, error) {
	account, err := p.accounts.GetByIdentifier(ctx, DefaultAssistantIdentifier)
	if err == nil {
		return p.ensureDefaultAssistantProfile(ctx, account)
	}
	if !isAppNotFound(err) {
		return model.User{}, err
	}

	legacy, legacyErr := p.accounts.GetByIdentifier(ctx, DefaultAssistantLegacyIdentifier)
	if legacyErr == nil {
		renamed, err := p.accounts.RenameIdentifier(ctx, legacy.Identifier, DefaultAssistantIdentifier)
		if err != nil {
			return model.User{}, err
		}
		return p.ensureDefaultAssistantProfile(ctx, renamed)
	}
	if !isAppNotFound(legacyErr) {
		return model.User{}, legacyErr
	}

	created, err := p.accounts.Create(ctx, model.User{
		Identifier:  DefaultAssistantIdentifier,
		DisplayName: DefaultAssistantDisplayName,
		Name:        DefaultAssistantIdentifier,
		Gender:      GenderUnknown,
		AccountType: model.AccountTypeAgent,
	})
	if err != nil {
		return model.User{}, err
	}
	return created, nil
}

func (p *DefaultAssistantProvisioner) ensureDefaultAssistantProfile(ctx context.Context, account model.User) (model.User, error) {
	if account.AccountType != model.AccountTypeAgent {
		return model.User{}, apperror.InvalidArgument("agent_creator account_type must be agent")
	}
	if account.DisplayName == DefaultAssistantDisplayName && account.Name == DefaultAssistantIdentifier {
		return account, nil
	}
	displayName := DefaultAssistantDisplayName
	name := DefaultAssistantIdentifier
	return p.accounts.UpdateProfile(ctx, account.AccountID, repository.AccountProfilePatch{
		DisplayName: &displayName,
		Name:        &name,
	})
}

func (p *DefaultAssistantProvisioner) ensureDefaultAssistantAgent(ctx context.Context, account model.User) (model.Agent, error) {
	agent, err := p.agents.GetAgentByIMUserID(ctx, account.AccountID)
	if err != nil {
		if !isAppNotFound(err) {
			return model.Agent{}, err
		}
		return p.agents.CreateAgent(ctx, model.Agent{
			AccountID:   account.AccountID,
			IMUserID:    account.AccountID,
			Name:        DefaultAssistantAgentName,
			Description: DefaultAssistantAgentDescription,
			Status:      model.AgentStatusActive,
			CreatedBy:   account.AccountID,
		})
	}
	if agent.Name != DefaultAssistantAgentName || agent.Description != DefaultAssistantAgentDescription {
		name := DefaultAssistantAgentName
		description := DefaultAssistantAgentDescription
		updated, updateErr := p.agents.UpdateAgent(ctx, agent.AgentID, repository.AgentPatch{
			Name:        &name,
			Description: &description,
		})
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

func (p *DefaultAssistantProvisioner) ensureDefaultAssistantPrompt(ctx context.Context, account model.User) (model.AgentPrompt, error) {
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
		CreatedBy:           account.AccountID,
	})
}

func (p *DefaultAssistantProvisioner) ensurePythonExecuteTool(ctx context.Context, account model.User) (model.AgentTool, error) {
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
		CreatedBy:        account.AccountID,
	})
}

func (p *DefaultAssistantProvisioner) ensureAgentCreateTool(ctx context.Context, account model.User) (model.AgentTool, error) {
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
		CreatedBy:        account.AccountID,
	})
}

func isAppNotFound(err error) bool {
	return apperror.From(err).Code == apperror.CodeNotFound
}
