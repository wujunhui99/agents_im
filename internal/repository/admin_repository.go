package repository

import (
	"context"

	"github.com/wujunhui99/agents_im/pkg/agentaudit"
	"github.com/wujunhui99/agents_im/pkg/model"
)

type AccountSearchFilter struct {
	Query string
	Limit int
}

type AgentRunFilter struct {
	Status string
	Limit  int
	Offset int
}

type AdminAccountRepository interface {
	GetByID(ctx context.Context, accountID string) (model.User, error)
	SearchAccounts(ctx context.Context, filter AccountSearchFilter) ([]model.User, error)
	CountAccounts(ctx context.Context) (int64, error)
}

type AdminMessageRepository interface {
	GetMessages(ctx context.Context, conversationID string, fromSeq, toSeq int64, limit int, order string) ([]Message, bool, int64, error)
	GetConversationSeqStates(ctx context.Context, userID string, conversationIDs []string) ([]ConversationSeqState, error)
	CountMessages(ctx context.Context) (int64, error)
	CountConversations(ctx context.Context) (int64, error)
	ListRecentConversationStates(ctx context.Context, limit int) ([]ConversationSeqState, error)
}

type AdminAgentAuditRepository interface {
	GetAgentRun(ctx context.Context, runID string) (agentaudit.AgentRun, error)
	ListAgentToolCallsByRunID(ctx context.Context, runID string) ([]agentaudit.AgentToolCall, error)
	ListAgentFileReadsByRunID(ctx context.Context, runID string) ([]agentaudit.AgentFileRead, error)
	ListAgentPythonExecsByRunID(ctx context.Context, runID string) ([]agentaudit.AgentPythonExec, error)
	ListAgentRuns(ctx context.Context, filter AgentRunFilter) ([]agentaudit.AgentRun, error)
	GetAgentRunByTraceID(ctx context.Context, traceID string) (agentaudit.AgentRun, error)
	CountAgentRuns(ctx context.Context, status string) (int64, error)
}
