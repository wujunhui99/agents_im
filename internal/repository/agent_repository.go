package repository

import (
	"context"

	"github.com/wujunhui99/agents_im/common/share/model"
)

type AgentPatch struct {
	Name        *string
	Description *string
}

type AgentListFilter struct {
	Status    string
	CreatedBy string
	Limit     int
	Offset    int
}

type AgentRepository interface {
	CreateAgent(ctx context.Context, agent model.Agent) (model.Agent, error)
	GetAgent(ctx context.Context, agentID string) (model.Agent, error)
	GetAgentByIMUserID(ctx context.Context, imUserID string) (model.Agent, error)
	ListAgents(ctx context.Context, filter AgentListFilter) ([]model.Agent, error)
	UpdateAgent(ctx context.Context, agentID string, patch AgentPatch) (model.Agent, error)
	UpdateAgentStatus(ctx context.Context, agentID string, status string) (model.Agent, error)
}
