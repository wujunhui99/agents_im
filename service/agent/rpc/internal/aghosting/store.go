// Package aghosting owns the agent-conversation-hosting data layer for agent-rpc:
// the agent_conversation_hosting table (which agent auto-hosts a conversation)
// and the agent_trigger_idempotency ledger (durable, TTL-preemptive idempotency
// for the trigger pipeline). Both are agent-domain self-owned tables (no
// cross-domain reads), so this replaces the keystone
// internal/repository.AgentConversationHostingRepository outright (issue #670,
// split from #616) — agent-rpc 不再 import internal 的 agent_hosting。
//
// Store is the data interface (goctl model backed in prod via ModelStore,
// in-memory in tests/demo via MemoryStore). orchestrator.ConversationHostingService
// depends on this interface.
package aghosting

import (
	"context"
	"strings"
	"time"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/service/agent/rpc/internal/model"
)

const (
	// AgentTriggerStatus* 是 agent_trigger_idempotency.status 的 owner 视图（单一来源在 model）。
	AgentTriggerStatusRunning   = model.AgentTriggerStatusRunning
	AgentTriggerStatusSucceeded = model.AgentTriggerStatusSucceeded
	AgentTriggerStatusFailed    = model.AgentTriggerStatusFailed

	// DefaultAgentTriggerRunningTTL 是 running 触发的默认抢占 TTL：超过该时长仍 running
	// 视为崩溃残留，可被新触发抢占。
	DefaultAgentTriggerRunningTTL = 10 * time.Minute
)

// AgentConversationHosting 是单个会话的自动托管行（哪个 agent 账号托管该会话）。
type AgentConversationHosting struct {
	ConversationID             string
	AgentAccountID             string
	Enabled                    bool
	AllowAgentMessageRecursion bool
	CreatedAt                  time.Time
	UpdatedAt                  time.Time
}

// AgentTriggerStartInput 是抢占一个触发 idempotency_key 的入参。
type AgentTriggerStartInput struct {
	IdempotencyKey     string
	ConversationID     string
	AgentAccountID     string
	TriggerServerMsgID string
	TriggerEventID     string
	RunningTTL         time.Duration
}

// AgentTriggerFinishInput 是把 running 触发推进到终态的入参。
type AgentTriggerFinishInput struct {
	IdempotencyKey      string
	Status              string
	ResponseServerMsgID string
	ErrorMessage        string
}

// Store 是 agent 托管 + 触发幂等的数据访问接口。prod 注入 ModelStore（goctl），单测注入
// MemoryStore / fake。
type Store interface {
	// UpsertAgentConversationHosting 按 conversation_id 写入/更新托管行，返回写入后的行。
	UpsertAgentConversationHosting(ctx context.Context, hosting AgentConversationHosting) (AgentConversationHosting, error)
	// GetAgentConversationHosting 读取会话的托管行；不存在返回 apperror.NotFound。
	GetAgentConversationHosting(ctx context.Context, conversationID string) (AgentConversationHosting, error)
	// TryStartAgentTrigger 抢占式占用触发 key（TTL 抢占语义）；成功占用返回 true，否则 false。
	TryStartAgentTrigger(ctx context.Context, input AgentTriggerStartInput) (bool, error)
	// FinishAgentTrigger 把 running 触发推进到终态；key 不存在或已是终态返回 apperror.NotFound。
	FinishAgentTrigger(ctx context.Context, input AgentTriggerFinishInput) error
}

// validateAgentConversationHosting 校验托管行入参（required + 长度上限 + 无 NUL/冒号），不做规范化。
func validateAgentConversationHosting(input AgentConversationHosting) error {
	if err := validateAgentHostingRequired(input.ConversationID, "conversation_id"); err != nil {
		return err
	}
	return validateAgentHostingComponentID(input.AgentAccountID, "agent_account_id")
}

// validateAgentTriggerStartInput 校验抢占入参并归一 TTL（业务默认值，非输入规范化）。
func validateAgentTriggerStartInput(input AgentTriggerStartInput) (AgentTriggerStartInput, error) {
	if err := validateAgentHostingRequired(input.IdempotencyKey, "idempotency_key"); err != nil {
		return AgentTriggerStartInput{}, err
	}
	if err := validateAgentHostingRequired(input.ConversationID, "conversation_id"); err != nil {
		return AgentTriggerStartInput{}, err
	}
	if err := validateAgentHostingComponentID(input.AgentAccountID, "agent_account_id"); err != nil {
		return AgentTriggerStartInput{}, err
	}
	if err := validateAgentHostingRequired(input.TriggerServerMsgID, "trigger_server_msg_id"); err != nil {
		return AgentTriggerStartInput{}, err
	}
	input.RunningTTL = normalizeAgentTriggerRunningTTL(input.RunningTTL)
	return input, nil
}

// validateAgentTriggerFinishInput 校验终态推进入参并清理互斥字段（业务语义：succeeded 必带
// response_server_msg_id 且无 error；failed 无 response）。
func validateAgentTriggerFinishInput(input AgentTriggerFinishInput) (AgentTriggerFinishInput, error) {
	if err := validateAgentHostingRequired(input.IdempotencyKey, "idempotency_key"); err != nil {
		return AgentTriggerFinishInput{}, err
	}
	switch input.Status {
	case AgentTriggerStatusSucceeded:
		if err := validateAgentHostingRequired(input.ResponseServerMsgID, "response_server_msg_id"); err != nil {
			return AgentTriggerFinishInput{}, err
		}
		input.ErrorMessage = ""
	case AgentTriggerStatusFailed:
		input.ResponseServerMsgID = ""
	default:
		return AgentTriggerFinishInput{}, apperror.InvalidArgument("agent trigger status must be succeeded or failed")
	}
	return input, nil
}

func normalizeAgentTriggerRunningTTL(ttl time.Duration) time.Duration {
	if ttl <= 0 {
		return DefaultAgentTriggerRunningTTL
	}
	if ttl < time.Millisecond {
		return time.Millisecond
	}
	return ttl
}

func validateAgentHostingRequired(value string, field string) error {
	if strings.TrimSpace(value) == "" {
		return apperror.InvalidArgument(field + " is required")
	}
	if len([]rune(value)) > 256 {
		return apperror.InvalidArgument(field + " must be 256 characters or fewer")
	}
	if strings.Contains(value, "\x00") {
		return apperror.InvalidArgument(field + " cannot contain NUL")
	}
	return nil
}

func validateAgentHostingComponentID(value string, field string) error {
	if err := validateAgentHostingRequired(value, field); err != nil {
		return err
	}
	if strings.Contains(value, ":") {
		return apperror.InvalidArgument(field + " cannot contain ':'")
	}
	return nil
}
