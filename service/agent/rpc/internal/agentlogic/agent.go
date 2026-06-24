package agentlogic

import (
	"context"
	"strings"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/pkg/model"
)

const (
	accountTypeAgent = "agent"

	AgentStatusDraft    = model.AgentStatusDraft
	AgentStatusActive   = model.AgentStatusActive
	AgentStatusDisabled = model.AgentStatusDisabled
	AgentStatusArchived = model.AgentStatusArchived
)

// AgentLogic 是 agent CRUD 业务逻辑（#606，脱 internal/logic.AgentLogic）。
type AgentLogic struct {
	agents   AgentStore
	accounts AccountReader
}

func NewAgentLogic(agents AgentStore, accounts AccountReader) *AgentLogic {
	return &AgentLogic{agents: agents, accounts: accounts}
}

type AgentInfo struct {
	AgentID     string `json:"agent_id"`
	AccountID   string `json:"account_id"`
	IMUserID    string `json:"im_user_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Status      string `json:"status"`
	CreatedBy   string `json:"created_by"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type CreateAgentRequest struct {
	AccountID   string
	IMUserID    string
	Name        string
	Description string
	Status      string
	CreatedBy   string
}

type UpdateAgentRequest struct {
	AgentID     string
	Name        *string
	Description *string
}

type UpdateAgentStatusRequest struct {
	AgentID string
	Status  string
}

type ListAgentsRequest struct {
	Status    string
	CreatedBy string
	Limit     int
	Offset    int
}

func (l *AgentLogic) CreateAgent(ctx context.Context, req CreateAgentRequest) (AgentInfo, error) {
	accountIDInput := req.AccountID
	accountIDField := "account_id"
	if strings.TrimSpace(accountIDInput) == "" {
		accountIDInput = req.IMUserID
		accountIDField = "im_user_id"
	}
	accountID, err := normalizeRequiredID(accountIDInput, accountIDField)
	if err != nil {
		return AgentInfo{}, err
	}
	createdBy, err := normalizeRequiredID(req.CreatedBy, "created_by")
	if err != nil {
		return AgentInfo{}, err
	}
	name, err := normalizeAgentName(req.Name)
	if err != nil {
		return AgentInfo{}, err
	}
	description, err := normalizeAgentDescription(req.Description)
	if err != nil {
		return AgentInfo{}, err
	}
	status, err := normalizeAgentStatus(req.Status, true)
	if err != nil {
		return AgentInfo{}, err
	}
	if status == "" {
		status = AgentStatusDisabled
	}
	if err := l.ensureAgentAccountType(ctx, accountID); err != nil {
		return AgentInfo{}, err
	}

	agent, err := l.agents.CreateAgent(ctx, model.Agent{
		AccountID:   accountID,
		IMUserID:    accountID,
		Name:        name,
		Description: description,
		Status:      status,
		CreatedBy:   createdBy,
	})
	if err != nil {
		return AgentInfo{}, err
	}
	return toAgentInfo(agent), nil
}

func (l *AgentLogic) GetAgent(ctx context.Context, agentID string) (AgentInfo, error) {
	agentID, err := normalizeRequiredID(agentID, "agent_id")
	if err != nil {
		return AgentInfo{}, err
	}
	agent, err := l.agents.GetAgent(ctx, agentID)
	if err != nil {
		return AgentInfo{}, err
	}
	return toAgentInfo(agent), nil
}

func (l *AgentLogic) ListAgents(ctx context.Context, req ListAgentsRequest) ([]AgentInfo, error) {
	status, err := normalizeAgentStatus(req.Status, true)
	if err != nil {
		return nil, err
	}
	createdBy := strings.TrimSpace(req.CreatedBy)
	if createdBy != "" {
		createdBy, err = normalizeRequiredID(createdBy, "created_by")
		if err != nil {
			return nil, err
		}
	}
	if req.Limit < 0 {
		return nil, apperror.InvalidArgument("limit must be greater than or equal to 0")
	}
	if req.Limit > 100 {
		return nil, apperror.InvalidArgument("limit must be 100 or fewer")
	}
	if req.Offset < 0 {
		return nil, apperror.InvalidArgument("offset must be greater than or equal to 0")
	}
	agents, err := l.agents.ListAgents(ctx, status, createdBy, req.Limit, req.Offset)
	if err != nil {
		return nil, err
	}
	result := make([]AgentInfo, 0, len(agents))
	for _, agent := range agents {
		result = append(result, toAgentInfo(agent))
	}
	return result, nil
}

func (l *AgentLogic) UpdateAgent(ctx context.Context, req UpdateAgentRequest) (AgentInfo, error) {
	agentID, err := normalizeRequiredID(req.AgentID, "agent_id")
	if err != nil {
		return AgentInfo{}, err
	}
	var namePtr, descPtr *string
	if req.Name != nil {
		name, err := normalizeAgentName(*req.Name)
		if err != nil {
			return AgentInfo{}, err
		}
		namePtr = &name
	}
	if req.Description != nil {
		description, err := normalizeAgentDescription(*req.Description)
		if err != nil {
			return AgentInfo{}, err
		}
		descPtr = &description
	}
	agent, err := l.agents.UpdateAgent(ctx, agentID, namePtr, descPtr)
	if err != nil {
		return AgentInfo{}, err
	}
	return toAgentInfo(agent), nil
}

func (l *AgentLogic) UpdateAgentStatus(ctx context.Context, req UpdateAgentStatusRequest) (AgentInfo, error) {
	agentID, err := normalizeRequiredID(req.AgentID, "agent_id")
	if err != nil {
		return AgentInfo{}, err
	}
	status, err := normalizeAgentStatus(req.Status, false)
	if err != nil {
		return AgentInfo{}, err
	}
	agent, err := l.agents.UpdateAgentStatus(ctx, agentID, status)
	if err != nil {
		return AgentInfo{}, err
	}
	return toAgentInfo(agent), nil
}

func (l *AgentLogic) ensureAgentAccountType(ctx context.Context, accountID string) error {
	if l.accounts == nil {
		return apperror.Internal("account reader is not configured")
	}
	account, err := l.accounts.GetByID(ctx, accountID)
	if err != nil {
		return err
	}
	if string(account.AccountType) != accountTypeAgent {
		return apperror.Forbidden("account_type must be " + accountTypeAgent)
	}
	return nil
}

func normalizeAgentName(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", apperror.InvalidArgument("name is required")
	}
	if len([]rune(value)) > 64 {
		return "", apperror.InvalidArgument("name must be 64 characters or fewer")
	}
	return value, nil
}

func normalizeAgentDescription(value string) (string, error) {
	value = strings.TrimSpace(value)
	if len([]rune(value)) > 512 {
		return "", apperror.InvalidArgument("description must be 512 characters or fewer")
	}
	return value, nil
}

func normalizeAgentStatus(value string, allowEmpty bool) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" && allowEmpty {
		return "", nil
	}
	switch value {
	case AgentStatusDraft, AgentStatusActive, AgentStatusDisabled, AgentStatusArchived:
		return value, nil
	default:
		return "", apperror.InvalidArgument("status must be draft, active, disabled, or archived")
	}
}

func toAgentInfo(agent model.Agent) AgentInfo {
	return AgentInfo{
		AgentID:     agent.AgentID,
		AccountID:   agent.AccountID,
		IMUserID:    agent.IMUserID,
		Name:        agent.Name,
		Description: agent.Description,
		Status:      agent.Status,
		CreatedBy:   agent.CreatedBy,
		CreatedAt:   formatTime(agent.CreatedAt),
		UpdatedAt:   formatTime(agent.UpdatedAt),
	}
}
