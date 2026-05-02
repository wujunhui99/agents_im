package repository

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/wujunhui99/agents_im/internal/apperror"
	"github.com/wujunhui99/agents_im/internal/idgen"
	"github.com/wujunhui99/agents_im/internal/model"
)

type MemoryAgentRepository struct {
	mu        sync.RWMutex
	agents    map[string]model.Agent
	accountID map[string]string
	now       func() time.Time
}

func NewMemoryAgentRepository() *MemoryAgentRepository {
	return &MemoryAgentRepository{
		agents:    make(map[string]model.Agent),
		accountID: make(map[string]string),
		now:       time.Now,
	}
}

func (r *MemoryAgentRepository) CreateAgent(_ context.Context, agent model.Agent) (model.Agent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	agent = agent.Clone()
	if _, exists := r.accountID[agent.AccountID]; exists {
		return model.Agent{}, apperror.AlreadyExists("agent already exists for account_id")
	}

	if agent.AgentID == "" {
		agentID, err := idgen.NewString()
		if err != nil {
			return model.Agent{}, err
		}
		agent.AgentID = agentID
	}
	now := r.now().UTC()
	agent.CreatedAt = now
	agent.UpdatedAt = now

	r.agents[agent.AgentID] = agent.Clone()
	r.accountID[agent.AccountID] = agent.AgentID
	return agent.Clone(), nil
}

func (r *MemoryAgentRepository) GetAgent(_ context.Context, agentID string) (model.Agent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	agent, exists := r.agents[agentID]
	if !exists {
		return model.Agent{}, apperror.NotFound("agent not found")
	}
	return agent.Clone(), nil
}

func (r *MemoryAgentRepository) GetAgentByIMUserID(_ context.Context, imUserID string) (model.Agent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	agentID, exists := r.accountID[imUserID]
	if !exists {
		return model.Agent{}, apperror.NotFound("agent not found")
	}
	return r.agents[agentID].Clone(), nil
}

func (r *MemoryAgentRepository) ListAgents(_ context.Context, filter AgentListFilter) ([]model.Agent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	agents := make([]model.Agent, 0, len(r.agents))
	for _, agent := range r.agents {
		if filter.Status != "" && agent.Status != filter.Status {
			continue
		}
		if filter.CreatedBy != "" && agent.CreatedBy != filter.CreatedBy {
			continue
		}
		agents = append(agents, agent.Clone())
	}

	sort.Slice(agents, func(i, j int) bool {
		if agents[i].CreatedAt.Equal(agents[j].CreatedAt) {
			return agents[i].AgentID < agents[j].AgentID
		}
		return agents[i].CreatedAt.Before(agents[j].CreatedAt)
	})

	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}
	if offset >= len(agents) {
		return []model.Agent{}, nil
	}
	agents = agents[offset:]
	if filter.Limit > 0 && filter.Limit < len(agents) {
		agents = agents[:filter.Limit]
	}
	return agents, nil
}

func (r *MemoryAgentRepository) UpdateAgent(_ context.Context, agentID string, patch AgentPatch) (model.Agent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	agent, exists := r.agents[agentID]
	if !exists {
		return model.Agent{}, apperror.NotFound("agent not found")
	}
	if patch.Name != nil {
		agent.Name = *patch.Name
	}
	if patch.Description != nil {
		agent.Description = *patch.Description
	}
	agent.UpdatedAt = r.now().UTC()
	r.agents[agentID] = agent.Clone()
	return agent.Clone(), nil
}

func (r *MemoryAgentRepository) UpdateAgentStatus(_ context.Context, agentID string, status string) (model.Agent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	agent, exists := r.agents[agentID]
	if !exists {
		return model.Agent{}, apperror.NotFound("agent not found")
	}
	agent.Status = status
	agent.UpdatedAt = r.now().UTC()
	r.agents[agentID] = agent.Clone()
	return agent.Clone(), nil
}
