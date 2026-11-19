package consumer

import (
	"context"
	"fmt"

	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/zeromicro/go-zero/core/logx"

	"github.com/wujunhui99/agents_im/pkg/messaging"
	"github.com/wujunhui99/agents_im/service/agent/rpc/internal/agentlogic"
)

// defaultAssistantIdentifier 是默认助手在 user 域的账号 identifier（user-rpc 负责建号）。
const defaultAssistantIdentifier = "agent_creator"

// UserCreatedHandler 消费 user.created.v1：新账号建成后，由 agent 域完成默认助手绑定
// （agent 行 + 提示词 + 工具）与好友关系建立。这是 user-rpc → agent-rpc / friends-rpc
// 两条同步调用边的替代，方向固定为 agent → user / friends，不再成环。
type UserCreatedHandler struct {
	assistant AssistantProvisioner
	accounts  AssistantAccountReader
	friends   FriendshipEnsurer
}

// AssistantProvisioner 是 agent 域的默认助手装配端口（*agentlogic.DefaultAssistantProvisioner）。
type AssistantProvisioner interface {
	EnsureDefaultAssistant(ctx context.Context, accountID string) (agentlogic.DefaultAssistantResult, error)
}

// AssistantAccountReader 解析默认助手账号（user 域，经 user-rpc）。
type AssistantAccountReader interface {
	AssistantAccountID(ctx context.Context) (string, error)
}

// FriendshipEnsurer 建立好友关系（friends 域，经 friends-rpc）。
type FriendshipEnsurer interface {
	EnsureFriendship(ctx context.Context, userID, friendID string) error
}

func NewUserCreatedHandler(assistant AssistantProvisioner, accounts AssistantAccountReader, friends FriendshipEnsurer) (*UserCreatedHandler, error) {
	if assistant == nil || accounts == nil || friends == nil {
		return nil, fmt.Errorf("user-created handler requires assistant, accounts and friends ports")
	}
	return &UserCreatedHandler{assistant: assistant, accounts: accounts, friends: friends}, nil
}

// HandleBatch is the kgo poll callback. Errors leave offsets uncommitted so the
// batch is retried; every step is idempotent (EnsureDefaultAssistant /
// EnsureFriendship), so at-least-once redelivery converges.
func (h *UserCreatedHandler) HandleBatch(ctx context.Context, records []*kgo.Record) error {
	for _, record := range records {
		event, err := messaging.UnmarshalUserCreatedEvent(record.Value)
		if err != nil {
			logx.WithContext(ctx).Errorf("agent: drop malformed user.created record topic=%s partition=%d offset=%d: %v",
				record.Topic, record.Partition, record.Offset, err)
			continue
		}
		// 只有人类/测试账号需要默认助手；agent 账号（含助手自己）跳过。
		if event.AccountType != "user" && event.AccountType != "test" {
			continue
		}
		if err := h.provision(ctx, event); err != nil {
			return fmt.Errorf("provision default assistant for account %s: %w", event.AccountID, err)
		}
	}
	return nil
}

func (h *UserCreatedHandler) provision(ctx context.Context, event messaging.UserCreatedEvent) error {
	assistantID, err := h.accounts.AssistantAccountID(ctx)
	if err != nil {
		return err
	}
	if assistantID == "" || assistantID == event.AccountID {
		return nil
	}
	if _, err := h.assistant.EnsureDefaultAssistant(ctx, assistantID); err != nil {
		return err
	}
	return h.friends.EnsureFriendship(ctx, event.AccountID, assistantID)
}
