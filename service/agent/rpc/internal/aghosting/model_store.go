package aghosting

import (
	"context"
	"errors"

	"github.com/zeromicro/go-zero/core/stores/postgres"
	"github.com/zeromicro/go-zero/core/stores/sqlx"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/service/agent/rpc/internal/model"
)

// ModelStore 是 Store 的 goctl model 实现（背靠 agent_conversation_hosting +
// agent_trigger_idempotency 两表）。
type ModelStore struct {
	hosting  model.AgentConversationHostingModel
	triggers model.AgentTriggerIdempotencyModel
}

var _ Store = (*ModelStore)(nil)

// NewModelStore 用数据源构建 model-backed Store。
func NewModelStore(dataSource string) *ModelStore {
	return NewModelStoreFromConn(postgres.New(dataSource))
}

// NewModelStoreFromConn 用已建连接构建 model-backed Store。
func NewModelStoreFromConn(conn sqlx.SqlConn) *ModelStore {
	return &ModelStore{
		hosting:  model.NewAgentConversationHostingModel(conn),
		triggers: model.NewAgentTriggerIdempotencyModel(conn),
	}
}

func (s *ModelStore) UpsertAgentConversationHosting(ctx context.Context, hosting AgentConversationHosting) (AgentConversationHosting, error) {
	if err := validateAgentConversationHosting(hosting); err != nil {
		return AgentConversationHosting{}, err
	}
	row, err := s.hosting.Upsert(ctx, &model.AgentConversationHosting{
		ConversationId:             hosting.ConversationID,
		AgentAccountId:             hosting.AgentAccountID,
		Enabled:                    hosting.Enabled,
		AllowAgentMessageRecursion: hosting.AllowAgentMessageRecursion,
	})
	if err != nil {
		if model.IsCheckViolation(err) {
			return AgentConversationHosting{}, apperror.InvalidArgument("invalid agent conversation hosting")
		}
		return AgentConversationHosting{}, err
	}
	return hostingFromModel(row), nil
}

func (s *ModelStore) GetAgentConversationHosting(ctx context.Context, conversationID string) (AgentConversationHosting, error) {
	if err := validateAgentHostingRequired(conversationID, "conversation_id"); err != nil {
		return AgentConversationHosting{}, err
	}
	row, err := s.hosting.FindOne(ctx, conversationID)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return AgentConversationHosting{}, apperror.NotFound("agent conversation hosting not found")
		}
		return AgentConversationHosting{}, err
	}
	return hostingFromModel(row), nil
}

func (s *ModelStore) TryStartAgentTrigger(ctx context.Context, input AgentTriggerStartInput) (bool, error) {
	input, err := validateAgentTriggerStartInput(input)
	if err != nil {
		return false, err
	}
	started, err := s.triggers.TryStart(ctx, &model.AgentTriggerIdempotency{
		IdempotencyKey:     input.IdempotencyKey,
		ConversationId:     input.ConversationID,
		AgentAccountId:     input.AgentAccountID,
		TriggerServerMsgId: input.TriggerServerMsgID,
		TriggerEventId:     input.TriggerEventID,
	}, input.RunningTTL.Milliseconds())
	if err != nil {
		if model.IsCheckViolation(err) {
			return false, apperror.InvalidArgument("invalid agent trigger idempotency input")
		}
		return false, err
	}
	return started, nil
}

func (s *ModelStore) FinishAgentTrigger(ctx context.Context, input AgentTriggerFinishInput) error {
	input, err := validateAgentTriggerFinishInput(input)
	if err != nil {
		return err
	}
	finished, err := s.triggers.Finish(ctx, input.IdempotencyKey, input.Status, input.ResponseServerMsgID, input.ErrorMessage)
	if err != nil {
		return err
	}
	if !finished {
		return apperror.NotFound("agent trigger idempotency key not found")
	}
	return nil
}

func hostingFromModel(row *model.AgentConversationHosting) AgentConversationHosting {
	return AgentConversationHosting{
		ConversationID:             row.ConversationId,
		AgentAccountID:             row.AgentAccountId,
		Enabled:                    row.Enabled,
		AllowAgentMessageRecursion: row.AllowAgentMessageRecursion,
		CreatedAt:                  row.CreatedAt.UTC(),
		UpdatedAt:                  row.UpdatedAt.UTC(),
	}
}
