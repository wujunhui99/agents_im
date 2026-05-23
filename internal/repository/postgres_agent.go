package repository

import (
	"context"
	"strings"
	"time"

	"github.com/wujunhui99/agents_im/internal/apperror"
	"github.com/wujunhui99/agents_im/internal/idgen"
	"github.com/wujunhui99/agents_im/internal/model"
)

type postgresAgentRow struct {
	AgentID     string    `db:"agent_id"`
	AccountID   string    `db:"account_id"`
	Name        string    `db:"name"`
	Description string    `db:"description"`
	Status      string    `db:"status"`
	CreatedBy   string    `db:"created_by"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}

func (r *PostgresRepository) CreateAgent(ctx context.Context, agent model.Agent) (model.Agent, error) {
	agent = agent.Clone()
	if strings.TrimSpace(agent.AgentID) == "" {
		agentID, err := idgen.NewString()
		if err != nil {
			return model.Agent{}, err
		}
		agent.AgentID = agentID
	}

	var row postgresAgentRow
	err := r.conn.QueryRowCtx(ctx, &row, `
insert into agents (agent_id, account_id, name, description, status, created_by)
values ($1, $2, $3, $4, $5, $6)
returning agent_id::text as agent_id, account_id, name, description, status, created_by, created_at, updated_at
`, agent.AgentID, agent.AccountID, agent.Name, agent.Description, agent.Status, agent.CreatedBy)
	if err != nil {
		if isPostgresUniqueViolation(err) {
			return model.Agent{}, apperror.AlreadyExists("agent already exists")
		}
		if isPostgresForeignKeyViolation(err) {
			return model.Agent{}, apperror.NotFound("agent account not found")
		}
		if isPostgresCheckViolation(err) {
			return model.Agent{}, apperror.InvalidArgument("invalid agent")
		}
		return model.Agent{}, err
	}
	return row.agent(), nil
}

func (r *PostgresRepository) GetAgent(ctx context.Context, agentID string) (model.Agent, error) {
	var row postgresAgentRow
	err := r.conn.QueryRowCtx(ctx, &row, `
select agent_id::text as agent_id, account_id, name, description, status, created_by, created_at, updated_at
from agents
where agent_id = $1::bigint
`, agentID)
	if err != nil {
		if isNotFound(err) {
			return model.Agent{}, apperror.NotFound("agent not found")
		}
		return model.Agent{}, err
	}
	return row.agent(), nil
}

func (r *PostgresRepository) GetAgentByIMUserID(ctx context.Context, imUserID string) (model.Agent, error) {
	var row postgresAgentRow
	err := r.conn.QueryRowCtx(ctx, &row, `
select agent_id::text as agent_id, account_id, name, description, status, created_by, created_at, updated_at
from agents
where account_id = $1
`, imUserID)
	if err != nil {
		if isNotFound(err) {
			return model.Agent{}, apperror.NotFound("agent not found")
		}
		return model.Agent{}, err
	}
	return row.agent(), nil
}

func (r *PostgresRepository) ListAgents(ctx context.Context, filter AgentListFilter) ([]model.Agent, error) {
	clauses := make([]string, 0, 2)
	args := make([]any, 0, 4)
	if filter.Status != "" {
		args = append(args, filter.Status)
		clauses = append(clauses, "status = $"+itoa(len(args)))
	}
	if filter.CreatedBy != "" {
		args = append(args, filter.CreatedBy)
		clauses = append(clauses, "created_by = $"+itoa(len(args)))
	}

	query := `
select agent_id::text as agent_id, account_id, name, description, status, created_by, created_at, updated_at
from agents
`
	if len(clauses) > 0 {
		query += "where " + strings.Join(clauses, " and ") + "\n"
	}
	query += "order by created_at asc, agent_id asc\n"
	if filter.Limit > 0 {
		args = append(args, filter.Limit)
		query += "limit $" + itoa(len(args)) + "\n"
	}
	if filter.Offset > 0 {
		args = append(args, filter.Offset)
		query += "offset $" + itoa(len(args)) + "\n"
	}

	var rows []postgresAgentRow
	if err := r.conn.QueryRowsCtx(ctx, &rows, query, args...); err != nil {
		return nil, err
	}

	agents := make([]model.Agent, 0, len(rows))
	for _, row := range rows {
		agents = append(agents, row.agent())
	}
	return agents, nil
}

func (r *PostgresRepository) UpdateAgent(ctx context.Context, agentID string, patch AgentPatch) (model.Agent, error) {
	setters := make([]string, 0, 2)
	args := make([]any, 0, 3)
	addSetter := func(column string, value any) {
		args = append(args, value)
		setters = append(setters, column+" = $"+itoa(len(args)))
	}

	if patch.Name != nil {
		addSetter("name", *patch.Name)
	}
	if patch.Description != nil {
		addSetter("description", *patch.Description)
	}
	if len(setters) == 0 {
		return r.GetAgent(ctx, agentID)
	}

	args = append(args, agentID)
	query := `
update agents
set ` + strings.Join(setters, ", ") + `, updated_at = now()
where agent_id = $` + itoa(len(args)) + `::bigint
returning agent_id::text as agent_id, account_id, name, description, status, created_by, created_at, updated_at
`
	var row postgresAgentRow
	if err := r.conn.QueryRowCtx(ctx, &row, query, args...); err != nil {
		if isNotFound(err) {
			return model.Agent{}, apperror.NotFound("agent not found")
		}
		if isPostgresCheckViolation(err) {
			return model.Agent{}, apperror.InvalidArgument("invalid agent")
		}
		return model.Agent{}, err
	}
	return row.agent(), nil
}

func (r *PostgresRepository) UpdateAgentStatus(ctx context.Context, agentID string, status string) (model.Agent, error) {
	var row postgresAgentRow
	if err := r.conn.QueryRowCtx(ctx, &row, `
update agents
set status = $2, updated_at = now()
where agent_id = $1::bigint
returning agent_id::text as agent_id, account_id, name, description, status, created_by, created_at, updated_at
`, agentID, status); err != nil {
		if isNotFound(err) {
			return model.Agent{}, apperror.NotFound("agent not found")
		}
		if isPostgresCheckViolation(err) {
			return model.Agent{}, apperror.InvalidArgument("invalid agent status")
		}
		return model.Agent{}, err
	}
	return row.agent(), nil
}

func (r postgresAgentRow) agent() model.Agent {
	return model.Agent{
		AgentID:     r.AgentID,
		AccountID:   r.AccountID,
		IMUserID:    r.AccountID,
		Name:        r.Name,
		Description: r.Description,
		Status:      r.Status,
		CreatedBy:   r.CreatedBy,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}
}
