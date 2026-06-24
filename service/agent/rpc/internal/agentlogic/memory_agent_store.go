package agentlogic

import (
	"context"
	"strconv"
	"sync"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/pkg/model"
)

// MemoryAgentStore 是 agents 表的内存实现，仅供单测 fixture 使用（替代旧
// internal/repository.MemoryAgentRepository，#606 退役 internal 后）。实现 orchestrator.AgentReader
// 所需 GetAgentByIMUserID + CRUD 子集。生产路径用 AgentStore（goctl model），绝不用本类型。
type MemoryAgentStore struct {
	mu     sync.Mutex
	seq    int64
	byID   map[string]model.Agent
	byAcct map[string]string
}

func NewMemoryAgentStore() *MemoryAgentStore {
	return &MemoryAgentStore{byID: map[string]model.Agent{}, byAcct: map[string]string{}}
}

func (s *MemoryAgentStore) CreateAgent(_ context.Context, agent model.Agent) (model.Agent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.byAcct[agent.AccountID]; ok {
		return model.Agent{}, apperror.AlreadyExists("agent already exists")
	}
	s.seq++
	agent.AgentID = strconv.FormatInt(s.seq, 10)
	agent.IMUserID = agent.AccountID
	s.byID[agent.AgentID] = agent
	s.byAcct[agent.AccountID] = agent.AgentID
	return agent, nil
}

func (s *MemoryAgentStore) GetAgent(_ context.Context, agentID string) (model.Agent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	agent, ok := s.byID[agentID]
	if !ok {
		return model.Agent{}, apperror.NotFound("agent not found")
	}
	return agent, nil
}

func (s *MemoryAgentStore) GetAgentByAccountID(_ context.Context, accountID string) (model.Agent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id, ok := s.byAcct[accountID]
	if !ok {
		return model.Agent{}, apperror.NotFound("agent not found")
	}
	return s.byID[id], nil
}

// GetAgentByIMUserID 满足 orchestrator.AgentReader（im_user_id == account_id）。
func (s *MemoryAgentStore) GetAgentByIMUserID(ctx context.Context, imUserID string) (model.Agent, error) {
	return s.GetAgentByAccountID(ctx, imUserID)
}

func (s *MemoryAgentStore) ListAgents(_ context.Context, status, createdBy string, limit, offset int) ([]model.Agent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// 稳定顺序：按自增 ID 升序（seq 从 1 起）。
	agents := make([]model.Agent, 0, len(s.byID))
	for id := int64(1); id <= s.seq; id++ {
		agent, ok := s.byID[strconv.FormatInt(id, 10)]
		if !ok {
			continue
		}
		if status != "" && agent.Status != status {
			continue
		}
		if createdBy != "" && agent.CreatedBy != createdBy {
			continue
		}
		agents = append(agents, agent)
	}
	if offset > 0 {
		if offset >= len(agents) {
			return []model.Agent{}, nil
		}
		agents = agents[offset:]
	}
	if limit > 0 && limit < len(agents) {
		agents = agents[:limit]
	}
	return agents, nil
}

func (s *MemoryAgentStore) UpdateAgent(_ context.Context, agentID string, name, description *string) (model.Agent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	agent, ok := s.byID[agentID]
	if !ok {
		return model.Agent{}, apperror.NotFound("agent not found")
	}
	if name != nil {
		agent.Name = *name
	}
	if description != nil {
		agent.Description = *description
	}
	s.byID[agentID] = agent
	return agent, nil
}

func (s *MemoryAgentStore) UpdateAgentStatus(_ context.Context, agentID string, status string) (model.Agent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	agent, ok := s.byID[agentID]
	if !ok {
		return model.Agent{}, apperror.NotFound("agent not found")
	}
	agent.Status = status
	s.byID[agentID] = agent
	return agent, nil
}
