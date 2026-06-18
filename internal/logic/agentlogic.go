package logic

import (
	"context"
	"strings"

	"github.com/wujunhui99/agents_im/pkg/model"
	"github.com/wujunhui99/agents_im/internal/repository"
	"github.com/wujunhui99/agents_im/pkg/apperror"
)

const (
	AccountTypeAgent = "agent"

	AgentStatusDraft    = model.AgentStatusDraft
	AgentStatusActive   = model.AgentStatusActive
	AgentStatusDisabled = model.AgentStatusDisabled
	AgentStatusArchived = model.AgentStatusArchived
)

type UserAccountTypeChecker interface {
	EnsureUserAccountType(ctx context.Context, userID string, accountType string) error
}

type AccountTypeChecker = UserAccountTypeChecker

type UserAccountTypeCheckerFunc func(ctx context.Context, userID string, accountType string) error

func (f UserAccountTypeCheckerFunc) EnsureUserAccountType(ctx context.Context, userID string, accountType string) error {
	if f == nil {
		return apperror.Internal("account_type checker is not configured")
	}
	return f(ctx, userID, accountType)
}

type FailClosedUserAccountTypeChecker struct{}

func NewFailClosedUserAccountTypeChecker() FailClosedUserAccountTypeChecker {
	return FailClosedUserAccountTypeChecker{}
}

func (FailClosedUserAccountTypeChecker) EnsureUserAccountType(context.Context, string, string) error {
	return apperror.Internal("account_type checker is not configured")
}

type UserLogicAccountTypeChecker struct {
	userLogic *UserLogic
}

func NewUserLogicAccountTypeChecker(userLogic *UserLogic) UserLogicAccountTypeChecker {
	return UserLogicAccountTypeChecker{userLogic: userLogic}
}

func (c UserLogicAccountTypeChecker) EnsureUserAccountType(ctx context.Context, userID string, accountType string) error {
	if c.userLogic == nil {
		return apperror.Internal("account_type checker is not configured")
	}
	profile, err := c.userLogic.GetUserByID(ctx, GetUserByIDRequest{UserID: userID})
	if err != nil {
		return err
	}
	if profile.AccountType != accountType {
		return apperror.Forbidden("account_type must be " + accountType)
	}
	return nil
}

type AgentLogic struct {
	repo               repository.AgentRepository
	accountTypeChecker UserAccountTypeChecker
}

func NewAgentLogic(repo repository.AgentRepository, accountTypeChecker UserAccountTypeChecker) *AgentLogic {
	return &AgentLogic{repo: repo, accountTypeChecker: accountTypeChecker}
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
	AccountID   string `json:"account_id"`
	IMUserID    string `json:"im_user_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Status      string `json:"status"`
	CreatedBy   string `json:"created_by"`
}

type AgentPathRequest struct {
	AgentID string `json:"agent_id"`
}

type UpdateAgentRequest struct {
	AgentID     string  `json:"agent_id"`
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
}

type UpdateAgentStatusRequest struct {
	AgentID string `json:"agent_id"`
	Status  string `json:"status"`
}

type ListAgentsRequest struct {
	Status    string `json:"status"`
	CreatedBy string `json:"created_by"`
	Limit     int    `json:"limit"`
	Offset    int    `json:"offset"`
}

type ListAgentsResponse struct {
	Agents []AgentInfo `json:"agents"`
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
	if err := l.ensureAgentRepository(); err != nil {
		return AgentInfo{}, err
	}
	if err := l.ensureAgentAccountType(ctx, accountID); err != nil {
		return AgentInfo{}, err
	}

	agent, err := l.repo.CreateAgent(ctx, model.Agent{
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

func (l *AgentLogic) GetAgent(ctx context.Context, req AgentPathRequest) (AgentInfo, error) {
	agentID, err := normalizeRequiredID(req.AgentID, "agent_id")
	if err != nil {
		return AgentInfo{}, err
	}
	if err := l.ensureAgentRepository(); err != nil {
		return AgentInfo{}, err
	}

	agent, err := l.repo.GetAgent(ctx, agentID)
	if err != nil {
		return AgentInfo{}, err
	}
	return toAgentInfo(agent), nil
}

func (l *AgentLogic) ListAgents(ctx context.Context, req ListAgentsRequest) (ListAgentsResponse, error) {
	status, err := normalizeAgentStatus(req.Status, true)
	if err != nil {
		return ListAgentsResponse{}, err
	}
	createdBy := strings.TrimSpace(req.CreatedBy)
	if createdBy != "" {
		createdBy, err = normalizeRequiredID(createdBy, "created_by")
		if err != nil {
			return ListAgentsResponse{}, err
		}
	}
	if req.Limit < 0 {
		return ListAgentsResponse{}, apperror.InvalidArgument("limit must be greater than or equal to 0")
	}
	if req.Limit > 100 {
		return ListAgentsResponse{}, apperror.InvalidArgument("limit must be 100 or fewer")
	}
	if req.Offset < 0 {
		return ListAgentsResponse{}, apperror.InvalidArgument("offset must be greater than or equal to 0")
	}
	if err := l.ensureAgentRepository(); err != nil {
		return ListAgentsResponse{}, err
	}

	agents, err := l.repo.ListAgents(ctx, repository.AgentListFilter{
		Status:    status,
		CreatedBy: createdBy,
		Limit:     req.Limit,
		Offset:    req.Offset,
	})
	if err != nil {
		return ListAgentsResponse{}, err
	}

	result := ListAgentsResponse{Agents: make([]AgentInfo, 0, len(agents))}
	for _, agent := range agents {
		result.Agents = append(result.Agents, toAgentInfo(agent))
	}
	return result, nil
}

func (l *AgentLogic) UpdateAgent(ctx context.Context, req UpdateAgentRequest) (AgentInfo, error) {
	agentID, err := normalizeRequiredID(req.AgentID, "agent_id")
	if err != nil {
		return AgentInfo{}, err
	}
	patch := repository.AgentPatch{}
	if req.Name != nil {
		name, err := normalizeAgentName(*req.Name)
		if err != nil {
			return AgentInfo{}, err
		}
		patch.Name = &name
	}
	if req.Description != nil {
		description, err := normalizeAgentDescription(*req.Description)
		if err != nil {
			return AgentInfo{}, err
		}
		patch.Description = &description
	}
	if err := l.ensureAgentRepository(); err != nil {
		return AgentInfo{}, err
	}

	agent, err := l.repo.UpdateAgent(ctx, agentID, patch)
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
	if err := l.ensureAgentRepository(); err != nil {
		return AgentInfo{}, err
	}

	agent, err := l.repo.UpdateAgentStatus(ctx, agentID, status)
	if err != nil {
		return AgentInfo{}, err
	}
	return toAgentInfo(agent), nil
}

func (l *AgentLogic) ArchiveAgent(ctx context.Context, req AgentPathRequest) (AgentInfo, error) {
	return l.UpdateAgentStatus(ctx, UpdateAgentStatusRequest{
		AgentID: req.AgentID,
		Status:  AgentStatusArchived,
	})
}

func (l *AgentLogic) ensureAgentRepository() error {
	if l.repo == nil {
		return apperror.Internal("agent repository is not configured")
	}
	return nil
}

func (l *AgentLogic) ensureAgentAccountType(ctx context.Context, imUserID string) error {
	if l.accountTypeChecker == nil {
		return apperror.Internal("account_type checker is not configured")
	}
	return l.accountTypeChecker.EnsureUserAccountType(ctx, imUserID, AccountTypeAgent)
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
	agent = agent.Clone()
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
