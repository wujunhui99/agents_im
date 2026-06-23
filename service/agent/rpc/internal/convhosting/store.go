// Package convhosting owns the conversation_ai_hosting_settings data layer and
// business rules for agent-rpc. It replaces the keystone internal/repository
// ConversationAIHostingRepository + internal/logic ConversationAIHostingLogic:
// the data owner is the agent domain (AG-6 ① / D13), the table is read/written
// only by agent-rpc, so the slice is deleted from internal/ outright (not a shim).
//
// Store is the data interface (model-backed in prod, memory-backed in tests/demo);
// ConversationAIHostingLogic (logic.go) is the gRPC CRUD business rule layer.
package convhosting

import (
	"context"
	"time"

	"github.com/wujunhui99/agents_im/pkg/apperror"
)

const (
	// modeAutoReply 是 V1 唯一支持的托管模式（schema check 约束 mode in ('auto_reply')）。
	modeAutoReply = "auto_reply"
	// defaultRecentMessages / maxRecentMessages 限定参与回复的最近消息条数（schema check 1..30）。
	defaultRecentMessages = 30
	maxRecentMessages     = 30
)

// Setting 是单个 owner 在单个单聊会话内的 AI 托管开关行（domain 视图）。
type Setting struct {
	OwnerAccountID    string
	ConversationID    string
	Enabled           bool
	Mode              string
	MaxRecentMessages int
	SummaryEnabled    bool
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// Update 是 SetConversationAIHostingEnabled 的入参。
type Update struct {
	OwnerAccountID    string
	ConversationID    string
	Enabled           bool
	MaxRecentMessages int
	SummaryEnabled    bool
}

// Store 是 conversation_ai_hosting_settings 的数据访问接口。
// trigger hosting.Store、orchestrator（request builder + ConversationHostingService）、
// gRPC CRUD Logic 均依赖本接口；prod 注入 model-backed 实现，单测注入内存实现/fake。
type Store interface {
	// GetConversationAIHostingSetting 读取 (owner, conversation) 的托管行；不存在返回 apperror.NotFound。
	GetConversationAIHostingSetting(ctx context.Context, ownerAccountID string, conversationID string) (Setting, error)
	// GetEnabledConversationAIHosting 读取会话内当前已开启的托管行；无开启行返回 apperror.NotFound。
	GetEnabledConversationAIHosting(ctx context.Context, conversationID string) (Setting, error)
	// SetConversationAIHostingEnabled 写入/更新托管开关；对端已开启时返回冲突错误。
	SetConversationAIHostingEnabled(ctx context.Context, input Update) (Setting, error)
}

// conflictError 是“对端已开启，本会话只能一方托管”的业务错误（partial unique index 冲突翻译）。
func conflictError() error {
	return apperror.AlreadyExists("对方已开启 AI 托管，本会话暂时只能由一方开启")
}

// clampRecentMessages 把最近消息条数夹到 [1, 30]，0/负值取默认 30。
func clampRecentMessages(value int) int {
	if value <= 0 {
		return defaultRecentMessages
	}
	if value > maxRecentMessages {
		return maxRecentMessages
	}
	return value
}
