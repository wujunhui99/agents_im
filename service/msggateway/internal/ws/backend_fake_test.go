package ws

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/pkg/gateway"
)

// fakeBackend 是测试用 in-memory MessageBackend：seq 按会话自增、
// client_msg_id 按 sender 去重、会话成员关系驱动 get_conversation_seqs，
// 语义对齐 msg-rpc（同步写、ACK 带 seq）。仅测试 fixture 使用。
type fakeBackend struct {
	mu            sync.Mutex
	nextServerID  int64
	conversations map[string]*fakeConversation
	dedup         map[string]gateway.MessageSnapshot
	hasRead       map[string]int64
	sendCalls     []gateway.SendMessageRPCRequest
}

type fakeConversation struct {
	messages []gateway.MessageSnapshot
	members  map[string]struct{}
}

func newFakeBackend() *fakeBackend {
	return &fakeBackend{
		conversations: make(map[string]*fakeConversation),
		dedup:         make(map[string]gateway.MessageSnapshot),
		hasRead:       make(map[string]int64),
	}
}

func (b *fakeBackend) SendMessage(_ context.Context, req gateway.SendMessageRPCRequest) (gateway.SendMessageRPCResponse, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.sendCalls = append(b.sendCalls, req)

	dedupKey := req.SenderID + "|" + req.ClientMsgID
	if existing, ok := b.dedup[dedupKey]; ok {
		return gateway.SendMessageRPCResponse{Message: existing, Deduplicated: true}, nil
	}

	conversationID, err := fakeConversationID(req)
	if err != nil {
		return gateway.SendMessageRPCResponse{}, err
	}
	conversation, ok := b.conversations[conversationID]
	if !ok {
		conversation = &fakeConversation{members: make(map[string]struct{})}
		b.conversations[conversationID] = conversation
	}
	conversation.members[req.SenderID] = struct{}{}
	if req.ReceiverID != "" {
		conversation.members[req.ReceiverID] = struct{}{}
	}

	b.nextServerID++
	now := time.Now().UnixMilli()
	message := gateway.MessageSnapshot{
		ServerMsgID:    fmt.Sprintf("srv_msg_%d", b.nextServerID),
		ClientMsgID:    req.ClientMsgID,
		ConversationID: conversationID,
		Seq:            int64(len(conversation.messages)) + 1,
		SenderID:       req.SenderID,
		ReceiverID:     req.ReceiverID,
		GroupID:        req.GroupID,
		ChatType:       req.ChatType,
		ContentType:    req.ContentType,
		Content:        req.Content,
		MessageOrigin:  "user",
		SendTime:       now,
		CreatedAt:      now,
	}
	conversation.messages = append(conversation.messages, message)
	b.dedup[dedupKey] = message
	return gateway.SendMessageRPCResponse{Message: message}, nil
}

func (b *fakeBackend) PullMessages(_ context.Context, req gateway.PullMessagesRPCRequest) (gateway.PullMessagesRPCResponse, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	conversation := b.conversations[req.ConversationID]
	if conversation == nil {
		return gateway.PullMessagesRPCResponse{IsEnd: true}, nil
	}
	maxSeq := int64(len(conversation.messages))
	fromSeq := req.FromSeq
	if fromSeq < 1 {
		fromSeq = 1
	}
	toSeq := req.ToSeq
	if toSeq <= 0 || toSeq > maxSeq {
		toSeq = maxSeq
	}
	limit := req.Limit
	if limit <= 0 {
		limit = 100
	}

	messages := make([]gateway.MessageSnapshot, 0)
	for seq := fromSeq; seq <= toSeq && int32(len(messages)) < limit; seq++ {
		messages = append(messages, conversation.messages[seq-1])
	}
	if req.Order == "desc" {
		for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
			messages[i], messages[j] = messages[j], messages[i]
		}
	}
	nextSeq := fromSeq + int64(len(messages))
	return gateway.PullMessagesRPCResponse{
		Messages: messages,
		IsEnd:    nextSeq > maxSeq,
		NextSeq:  nextSeq,
	}, nil
}

func (b *fakeBackend) GetConversationSeqs(_ context.Context, req gateway.GetConversationSeqsRPCRequest) (gateway.GetConversationSeqsRPCResponse, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	conversationIDs := req.ConversationIDs
	if len(conversationIDs) == 0 {
		for id, conversation := range b.conversations {
			if _, ok := conversation.members[req.UserID]; ok {
				conversationIDs = append(conversationIDs, id)
			}
		}
		sort.Strings(conversationIDs)
	}

	states := make([]gateway.ConversationSeqState, 0, len(conversationIDs))
	for _, id := range conversationIDs {
		conversation := b.conversations[id]
		if conversation == nil || len(conversation.messages) == 0 {
			continue
		}
		last := conversation.messages[len(conversation.messages)-1]
		hasRead := b.hasRead[req.UserID+"|"+id]
		states = append(states, gateway.ConversationSeqState{
			ConversationID: id,
			MaxSeq:         last.Seq,
			HasReadSeq:     hasRead,
			UnreadCount:    last.Seq - hasRead,
			MaxSeqTime:     last.SendTime,
			LastMessage:    &last,
		})
	}
	return gateway.GetConversationSeqsRPCResponse{States: states}, nil
}

func (b *fakeBackend) MarkConversationAsRead(_ context.Context, req gateway.MarkConversationAsReadRPCRequest) (gateway.MarkConversationAsReadRPCResponse, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	conversation := b.conversations[req.ConversationID]
	if conversation == nil {
		return gateway.MarkConversationAsReadRPCResponse{}, apperror.NotFound("conversation not found")
	}
	maxSeq := int64(len(conversation.messages))
	hasReadSeq := req.HasReadSeq
	if hasReadSeq > maxSeq {
		hasReadSeq = maxSeq
	}
	key := req.UserID + "|" + req.ConversationID
	updated := hasReadSeq > b.hasRead[key]
	if updated {
		b.hasRead[key] = hasReadSeq
	}
	return gateway.MarkConversationAsReadRPCResponse{
		ConversationID: req.ConversationID,
		HasReadSeq:     b.hasRead[key],
		MaxSeq:         maxSeq,
		UnreadCount:    maxSeq - b.hasRead[key],
		Updated:        updated,
	}, nil
}

func (b *fakeBackend) SendCalls() []gateway.SendMessageRPCRequest {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]gateway.SendMessageRPCRequest(nil), b.sendCalls...)
}

func fakeConversationID(req gateway.SendMessageRPCRequest) (string, error) {
	switch req.ChatType {
	case "group":
		if req.GroupID == "" {
			return "", apperror.InvalidArgument("groupId is required")
		}
		return "group:" + req.GroupID, nil
	case "single":
		if req.ReceiverID == "" {
			return "", apperror.InvalidArgument("receiverId is required")
		}
		users := []string{req.SenderID, req.ReceiverID}
		sort.Strings(users)
		return "single:" + users[0] + ":" + users[1], nil
	default:
		return "", apperror.InvalidArgument("unsupported chatType")
	}
}
