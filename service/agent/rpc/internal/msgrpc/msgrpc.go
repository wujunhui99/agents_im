// Package msgrpc 实现 AI 托管 runtime 的跨域读端口（#617，脱顶层 internal/{logic,repository}
// 的 message/groups keystone）：
//   - MessageHistory  → orchestrator.MessageHistoryReader：会话最近 N 条历史经 msg-rpc PullMessages；
//   - ReadAdvancer    → orchestrator.ConversationReadAdvancer：已读推进经 msg-rpc MarkConversationAsRead；
//   - GroupMembers    → orchestrator.GroupMemberLister：群成员鉴权经 groups-rpc ListMembers。
//
// 三者均为单向叶子调用（agent-rpc → msg-rpc / groups-rpc），属 runtime 编排，不在 rpc 间成环
// （groups-rpc 无下游 rpc 依赖；msg-rpc 仅单向调 media/groups）。错误经 rpcerror.FromStatus 还原 apperror。
package msgrpc

import (
	"context"

	"github.com/wujunhui99/agents_im/pkg/rpcerror"
	"github.com/wujunhui99/agents_im/service/agent/rpc/internal/orchestrator"
	groupspb "github.com/wujunhui99/agents_im/service/groups/rpc/groups"
	"github.com/wujunhui99/agents_im/service/groups/rpc/groupsclient"
	msgpb "github.com/wujunhui99/agents_im/service/msg/rpc/msg"
	"github.com/wujunhui99/agents_im/service/msg/rpc/msgclient"
)

// MessageHistory 把 msg-rpc PullMessages 暴露为 orchestrator.MessageHistoryReader。
// AI 托管仅单聊、以 agent 账号（会话参与者）视角拉取，PullMessages 的读鉴权对参与者天然通过。
type MessageHistory struct {
	rpc msgclient.Msg
}

func NewMessageHistory(rpc msgclient.Msg) *MessageHistory {
	return &MessageHistory{rpc: rpc}
}

func (h *MessageHistory) GetRecentMessages(ctx context.Context, req orchestrator.RecentMessagesRequest) ([]orchestrator.Message, error) {
	resp, err := h.rpc.PullMessages(ctx, &msgpb.PullMessagesRequest{
		UserId:         req.UserID,
		ConversationId: req.ConversationID,
		FromSeq:        req.FromSeq,
		ToSeq:          req.ToSeq,
		Limit:          int32(req.Limit),
		Order:          req.Order,
	})
	if err != nil {
		return nil, rpcerror.FromStatus(err)
	}
	messages := make([]orchestrator.Message, 0, len(resp.GetMessages()))
	for _, m := range resp.GetMessages() {
		messages = append(messages, messageFromPB(m))
	}
	return messages, nil
}

func messageFromPB(m *msgpb.Message) orchestrator.Message {
	if m == nil {
		return orchestrator.Message{}
	}
	return orchestrator.Message{
		ServerMsgID:           m.GetServerMsgId(),
		ClientMsgID:           m.GetClientMsgId(),
		ConversationID:        m.GetConversationId(),
		Seq:                   m.GetSeq(),
		SenderID:              m.GetSenderId(),
		ReceiverID:            m.GetReceiverId(),
		GroupID:               m.GetGroupId(),
		ChatType:              m.GetChatType(),
		ContentType:           m.GetContentType(),
		Content:               m.GetContent(),
		MessageOrigin:         m.GetMessageOrigin(),
		AgentAccountID:        m.GetAgentAccountId(),
		TriggerServerMsgID:    m.GetTriggerServerMsgId(),
		AgentRunID:            m.GetAgentRunId(),
		AllowRecursiveTrigger: m.GetAllowRecursiveTrigger(),
		SendTime:              m.GetSendTime(),
		CreatedAt:             m.GetCreatedAt(),
	}
}

// ReadAdvancer 把 msg-rpc MarkConversationAsRead 暴露为 orchestrator.ConversationReadAdvancer。
type ReadAdvancer struct {
	rpc msgclient.Msg
}

func NewReadAdvancer(rpc msgclient.Msg) *ReadAdvancer {
	return &ReadAdvancer{rpc: rpc}
}

func (a *ReadAdvancer) MarkConversationRead(ctx context.Context, accountID, conversationID string, seq int64) error {
	if _, err := a.rpc.MarkConversationAsRead(ctx, &msgpb.MarkConversationAsReadRequest{
		UserId:         accountID,
		ConversationId: conversationID,
		HasReadSeq:     seq,
	}); err != nil {
		return rpcerror.FromStatus(err)
	}
	return nil
}

// GroupMembers 把 groups-rpc ListMembers 暴露为 orchestrator.GroupMemberLister。
type GroupMembers struct {
	rpc groupsclient.Groups
}

func NewGroupMembers(rpc groupsclient.Groups) *GroupMembers {
	return &GroupMembers{rpc: rpc}
}

func (g *GroupMembers) ListMembers(ctx context.Context, req orchestrator.ListMembersRequest) (orchestrator.ListMembersResponse, error) {
	resp, err := g.rpc.ListMembers(ctx, &groupspb.ListMembersRequest{
		GroupId:         req.GroupID,
		RequesterUserId: req.RequesterUserID,
	})
	if err != nil {
		return orchestrator.ListMembersResponse{}, rpcerror.FromStatus(err)
	}
	members := make([]orchestrator.GroupMemberInfo, 0, len(resp.GetMembers()))
	for _, m := range resp.GetMembers() {
		members = append(members, orchestrator.GroupMemberInfo{
			GroupID: m.GetGroupId(),
			UserID:  m.GetUserId(),
			Role:    m.GetRole(),
			State:   m.GetState(),
		})
	}
	return orchestrator.ListMembersResponse{GroupID: resp.GetGroupId(), Members: members}, nil
}
