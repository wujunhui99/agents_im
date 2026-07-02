package orchestrator

import "context"

// 本文件是 AI 托管 runtime 的本地 DTO/端口，取代顶层 internal/{logic,repository} 的
// message/groups 类型（#617：runtime message/groups 读改走 owner gRPC）。历史读经 msg-rpc
// PullMessages、已读推进经 msg-rpc MarkConversationAsRead、群成员鉴权经 groups-rpc ListMembers，
// 由 service/agent/rpc/internal/msgrpc 适配器实现下列接口。

// 会话/消息枚举（值对齐 message 域 DB 常量，脱 internal/repository.message_enums）。
const (
	MessageChatTypeSingle = "single"
	MessageChatTypeGroup  = "group"

	MessageContentTypeText  = "text"
	MessageContentTypeImage = "image"
	MessageContentTypeFile  = "file"

	MessageOriginHuman  = "human"
	MessageOriginAI     = "ai"
	MessageOriginSystem = "system"
)

// Message 是 runtime 使用的消息视图（脱 internal/repository.Message，#617）。
type Message struct {
	ServerMsgID           string
	ClientMsgID           string
	ConversationID        string
	Seq                   int64
	SenderID              string
	ReceiverID            string
	GroupID               string
	ChatType              string
	ContentType           string
	Content               string
	MessageOrigin         string
	AgentAccountID        string
	TriggerServerMsgID    string
	AgentRunID            string
	AllowRecursiveTrigger bool
	SendTime              int64
	CreatedAt             int64
}

// SendMessageRequest / SendMessageResponse 是 AI 写回的请求/响应视图
// （脱 internal/logic.SendMessage*，由 imadapter.MsgRPCSender 经 msg-rpc gRPC SendMessage 承接）。
type SendMessageRequest struct {
	SenderID              string
	ReceiverID            string
	GroupID               string
	ChatType              string
	ClientMsgID           string
	ContentType           string
	Content               string
	MessageOrigin         string
	AgentAccountID        string
	TriggerServerMsgID    string
	AgentRunID            string
	AllowRecursiveTrigger bool
}

type SendMessageResponse struct {
	Message      Message
	Deduplicated bool
}

// MessageHistoryReader 读会话最近 N 条历史，喂请求构建器（脱 internal/repository.MessageRepository.GetMessages，
// 经 msg-rpc PullMessages 按 agent 参与者视角拉取）。
type MessageHistoryReader interface {
	GetRecentMessages(ctx context.Context, req RecentMessagesRequest) ([]Message, error)
}

type RecentMessagesRequest struct {
	// UserID 是拉取视角（AI 托管仅单聊，取 agent 账号——会话参与者）。
	UserID         string
	ConversationID string
	FromSeq        int64
	ToSeq          int64
	Limit          int
	Order          string
}

// GroupMemberLister 列群成员做鉴权（脱 internal/logic.GroupMemberLister，经 groups-rpc ListMembers）。
type GroupMemberLister interface {
	ListMembers(ctx context.Context, req ListMembersRequest) (ListMembersResponse, error)
}

type ListMembersRequest struct {
	GroupID         string
	RequesterUserID string
}

type ListMembersResponse struct {
	GroupID string
	Members []GroupMemberInfo
}

type GroupMemberInfo struct {
	GroupID string
	UserID  string
	Role    string
	State   string
}

// MessageCreatedHookInput 保留进程内消息钩子入参（历史路径；agent-rpc 生产走 Kafka 触发消费，
// 该钩子仅供测试/兼容，脱 internal/logic.MessageCreatedHookInput）。
type MessageCreatedHookInput struct {
	EventID               string
	OperationID           string
	TraceID               string
	Message               Message
	Deduplicated          bool
	RecipientUserIDs      []string
	TargetAgentAccountIDs []string
}
