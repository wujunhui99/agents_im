package logic

import (
	"context"
	"encoding/json"
	"sort"

	business "github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/pkg/observability"
	"github.com/wujunhui99/agents_im/service/msg/rpc/internal/model"
	"github.com/wujunhui99/agents_im/service/msg/rpc/msg"
)

// messageToPB 把存库行（goctl Messages）映射成对外 proto Message：int 枚举→string、content 解码、send_time coalesce。
func messageToPB(m *model.Messages) *msg.Message {
	if m == nil {
		return nil
	}
	return &msg.Message{
		ServerMsgId:           m.MessageId,
		ClientMsgId:           m.ClientMsgId,
		ConversationId:        m.ConversationId,
		Seq:                   m.Seq,
		SenderId:              m.SenderAccountId,
		ReceiverId:            m.ReceiverAccountId,
		GroupId:               m.GroupId,
		ChatType:              model.ConversationTypeString(m.ConversationType),
		ContentType:           model.ContentTypeString(m.ContentType),
		Content:               model.DecodeMessageContent(m.Content),
		MessageOrigin:         model.MessageOriginString(m.MessageOrigin),
		AgentAccountId:        m.AgentAccountId,
		TriggerServerMsgId:    m.TriggerMessageId,
		AgentRunId:            m.AgentRunId,
		AllowRecursiveTrigger: m.AllowRecursiveTrigger,
		SendTime:              model.MessageSendTime(m),
		CreatedAt:             m.ServerReceivedAt.UTC().UnixMilli(),
	}
}

// messageToBusiness 把存库行映射成 internal 层 Message（AI 托管钩子输入；keystone 例外，
// 待 03-message-pipeline §9 B1 把触发点迁到 msgtransfer 后随钩子一起删除）。字段语义与 messageToPB 一致。
func messageToBusiness(m *model.Messages) business.Message {
	if m == nil {
		return business.Message{}
	}
	return business.Message{
		ServerMsgID:           m.MessageId,
		ClientMsgID:           m.ClientMsgId,
		ConversationID:        m.ConversationId,
		Seq:                   m.Seq,
		SenderID:              m.SenderAccountId,
		ReceiverID:            m.ReceiverAccountId,
		GroupID:               m.GroupId,
		ChatType:              model.ConversationTypeString(m.ConversationType),
		ContentType:           model.ContentTypeString(m.ContentType),
		Content:               model.DecodeMessageContent(m.Content),
		MessageOrigin:         model.MessageOriginString(m.MessageOrigin),
		AgentAccountID:        m.AgentAccountId,
		TriggerServerMsgID:    m.TriggerMessageId,
		AgentRunID:            m.AgentRunId,
		AllowRecursiveTrigger: m.AllowRecursiveTrigger,
		SendTime:              model.MessageSendTime(m),
		CreatedAt:             m.ServerReceivedAt.UTC().UnixMilli(),
	}
}

func aiHostingStateToPB(s business.ConversationAIHostingResponse) *msg.ConversationAIHostingState {
	return &msg.ConversationAIHostingState{
		ConversationId:    s.ConversationID,
		ChatType:          s.ChatType,
		Enabled:           s.Enabled,
		Available:         s.Available,
		PeerEnabled:       s.PeerEnabled,
		UnavailableReason: s.UnavailableReason,
		MaxRecentMessages: int64(s.MaxRecentMessages),
		SummaryEnabled:    s.SummaryEnabled,
	}
}

func seqStateToPB(s model.ConversationSeqState) *msg.ConversationSeqState {
	out := &msg.ConversationSeqState{
		ConversationId: s.ConversationID,
		MaxSeq:         s.MaxSeq,
		HasReadSeq:     s.HasReadSeq,
		UnreadCount:    s.UnreadCount,
		MaxSeqTime:     s.MaxSeqTime,
	}
	if s.LastMessage != nil {
		out.LastMessage = messageToPB(s.LastMessage)
	}
	return out
}

// ---- message.created outbox 事件 payload ----
// 必须与 msgtransfer 消费方 (internal/outboxpublisher.MessageEventFromOutbox →
// repository.MessageCreatedOutboxPayload) 的 JSON 形状逐字节一致。

type outboxMessage struct {
	ServerMsgID           string `json:"serverMsgId"`
	ClientMsgID           string `json:"clientMsgId"`
	ConversationID        string `json:"conversationId"`
	Seq                   int64  `json:"seq"`
	SenderID              string `json:"senderId"`
	ReceiverID            string `json:"receiverId"`
	GroupID               string `json:"groupId"`
	ChatType              string `json:"chatType"`
	ContentType           string `json:"contentType"`
	Content               string `json:"content"`
	MessageOrigin         string `json:"messageOrigin"`
	AgentAccountID        string `json:"agentAccountId,omitempty"`
	TriggerServerMsgID    string `json:"triggerServerMsgId,omitempty"`
	AgentRunID            string `json:"agentRunId,omitempty"`
	AllowRecursiveTrigger bool   `json:"allowRecursiveTrigger,omitempty"`
	SendTime              int64  `json:"sendTime"`
	CreatedAt             int64  `json:"createdAt"`
}

type outboxTraceMetadata struct {
	TraceID     string `json:"trace_id,omitempty"`
	RequestID   string `json:"request_id,omitempty"`
	TraceParent string `json:"traceparent,omitempty"`
	TraceState  string `json:"tracestate,omitempty"`
}

type messageCreatedOutboxPayload struct {
	Message        outboxMessage       `json:"message"`
	VisibleUserIDs []string            `json:"visible_user_ids"`
	TraceContext   outboxTraceMetadata `json:"trace_context,omitempty"`
}

func buildMessageCreatedOutboxPayload(ctx context.Context, m *model.Messages, visibleUserIDs []string) (string, error) {
	visible := append([]string(nil), visibleUserIDs...)
	sort.Strings(visible)
	trace := observability.TraceContextFromContext(ctx)
	payload := messageCreatedOutboxPayload{
		Message: outboxMessage{
			ServerMsgID:           m.MessageId,
			ClientMsgID:           m.ClientMsgId,
			ConversationID:        m.ConversationId,
			Seq:                   m.Seq,
			SenderID:              m.SenderAccountId,
			ReceiverID:            m.ReceiverAccountId,
			GroupID:               m.GroupId,
			ChatType:              model.ConversationTypeString(m.ConversationType),
			ContentType:           model.ContentTypeString(m.ContentType),
			Content:               model.DecodeMessageContent(m.Content),
			MessageOrigin:         model.MessageOriginString(m.MessageOrigin),
			AgentAccountID:        m.AgentAccountId,
			TriggerServerMsgID:    m.TriggerMessageId,
			AgentRunID:            m.AgentRunId,
			AllowRecursiveTrigger: m.AllowRecursiveTrigger,
			SendTime:              model.MessageSendTime(m),
			CreatedAt:             m.ServerReceivedAt.UTC().UnixMilli(),
		},
		VisibleUserIDs: visible,
		TraceContext: outboxTraceMetadata{
			TraceID:     trace.TraceID,
			RequestID:   trace.RequestID,
			TraceParent: trace.TraceParent,
			TraceState:  trace.TraceState,
		},
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}
