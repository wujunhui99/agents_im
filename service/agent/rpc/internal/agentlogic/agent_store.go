package agentlogic

import (
	"context"
	"strconv"
	"strings"

	"github.com/zeromicro/go-zero/core/stores/postgres"
	"github.com/zeromicro/go-zero/core/stores/sqlx"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/pkg/model"
	rpcmodel "github.com/wujunhui99/agents_im/service/agent/rpc/internal/model"
)

// PgAgentStore 是 agents 表的 goctl-model 数据访问（#606，脱 internal/repository.PostgresRepository），
// 实现 AgentStore 接口。对外 string agent_id，内部 string↔int64 转换（agent_id 在 #013 已迁 bigint）。
type PgAgentStore struct {
	agents rpcmodel.AgentsModel
}

// NewAgentStore 用数据源构建。
func NewAgentStore(dataSource string) *PgAgentStore {
	return NewAgentStoreFromConn(postgres.New(dataSource))
}

// NewAgentStoreFromConn 用已建连接构建。
func NewAgentStoreFromConn(conn sqlx.SqlConn) *PgAgentStore {
	return &PgAgentStore{agents: rpcmodel.NewAgentsModel(conn)}
}

func (s *PgAgentStore) CreateAgent(ctx context.Context, agent model.Agent) (model.Agent, error) {
	row, err := s.agents.InsertReturning(ctx, &rpcmodel.Agents{
		AccountId:   agent.AccountID,
		Name:        agent.Name,
		Description: agent.Description,
		Status:      agent.Status,
		CreatedBy:   agent.CreatedBy,
	})
	if err != nil {
		if rpcmodel.IsUniqueViolation(err) {
			return model.Agent{}, apperror.AlreadyExists("agent already exists")
		}
		if rpcmodel.IsForeignKeyViolation(err) {
			return model.Agent{}, apperror.NotFound("agent account not found")
		}
		if rpcmodel.IsCheckViolation(err) {
			return model.Agent{}, apperror.InvalidArgument("invalid agent")
		}
		return model.Agent{}, err
	}
	return agentFromModel(row), nil
}

func (s *PgAgentStore) GetAgent(ctx context.Context, agentID string) (model.Agent, error) {
	id, ok := parseID(agentID)
	if !ok {
		return model.Agent{}, apperror.NotFound("agent not found")
	}
	row, err := s.agents.FindOne(ctx, id)
	if err != nil {
		return model.Agent{}, mapAgentNotFound(err)
	}
	return agentFromModel(row), nil
}

func (s *PgAgentStore) GetAgentByAccountID(ctx context.Context, accountID string) (model.Agent, error) {
	row, err := s.agents.FindOneByAccountId(ctx, accountID)
	if err != nil {
		return model.Agent{}, mapAgentNotFound(err)
	}
	return agentFromModel(row), nil
}

// GetAgentByIMUserID 满足 orchestrator.AgentReader（im_user_id == account_id）。
func (s *PgAgentStore) GetAgentByIMUserID(ctx context.Context, imUserID string) (model.Agent, error) {
	return s.GetAgentByAccountID(ctx, imUserID)
}

func (s *PgAgentStore) ListAgents(ctx context.Context, status, createdBy string, limit, offset int) ([]model.Agent, error) {
	rows, err := s.agents.ListFiltered(ctx, status, createdBy, limit, offset)
	if err != nil {
		return nil, err
	}
	agents := make([]model.Agent, 0, len(rows))
	for _, row := range rows {
		agents = append(agents, agentFromModel(row))
	}
	return agents, nil
}

func (s *PgAgentStore) UpdateAgent(ctx context.Context, agentID string, name, description *string) (model.Agent, error) {
	id, ok := parseID(agentID)
	if !ok {
		return model.Agent{}, apperror.NotFound("agent not found")
	}
	row, err := s.agents.UpdateNameDescription(ctx, id, name, description)
	if err != nil {
		if rpcmodel.IsCheckViolation(err) {
			return model.Agent{}, apperror.InvalidArgument("invalid agent")
		}
		return model.Agent{}, mapAgentNotFound(err)
	}
	return agentFromModel(row), nil
}

func (s *PgAgentStore) UpdateAgentStatus(ctx context.Context, agentID string, status string) (model.Agent, error) {
	id, ok := parseID(agentID)
	if !ok {
		return model.Agent{}, apperror.NotFound("agent not found")
	}
	row, err := s.agents.UpdateStatus(ctx, id, status)
	if err != nil {
		if rpcmodel.IsCheckViolation(err) {
			return model.Agent{}, apperror.InvalidArgument("invalid agent status")
		}
		return model.Agent{}, mapAgentNotFound(err)
	}
	return agentFromModel(row), nil
}

func agentFromModel(row *rpcmodel.Agents) model.Agent {
	accountID := row.AccountId
	return model.Agent{
		AgentID:     strconv.FormatInt(row.AgentId, 10),
		AccountID:   accountID,
		IMUserID:    accountID,
		Name:        row.Name,
		Description: row.Description,
		Status:      row.Status,
		CreatedBy:   row.CreatedBy,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}
}

func mapAgentNotFound(err error) error {
	if err == rpcmodel.ErrNotFound {
		return apperror.NotFound("agent not found")
	}
	return err
}

func parseID(value string) (int64, bool) {
	id, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return 0, false
	}
	return id, true
}
