package logic

import (
	"context"

	"github.com/wujunhui99/agents_im/internal/apperror"
	"github.com/wujunhui99/agents_im/internal/model"
	"github.com/wujunhui99/agents_im/internal/repository"
)

const (
	DefaultAssistantIdentifier       = "agent_creator"
	DefaultAssistantLegacyIdentifier = "agent_father"
	DefaultAssistantDisplayName      = "AI 助手"
	DefaultAssistantAgentName        = DefaultAssistantIdentifier
	DefaultAssistantAgentDescription = "Default general AI assistant"
	DefaultAssistantPromptName       = "agent_creator_default_system_prompt"
	DefaultAssistantPromptVersion    = "v1"
	DefaultAssistantSystemPrompt     = "你是一个通用 AI 助手，回答应准确、简洁、友好。你可以帮助用户解释概念、比较方案、整理信息、生成文本和提供编程/产品建议。不要编造事实；不确定时说明不确定并给出可验证的下一步。"
)

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
	if account.AccountType != model.AccountTypeUser {
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

func isAppNotFound(err error) bool {
	return apperror.From(err).Code == apperror.CodeNotFound
}
