package repository

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/wujunhui99/agents_im/internal/apperror"
)

type MemoryMessageRepository struct {
	mu            sync.RWMutex
	nextMessageID uint64
	conversations map[string]*memoryConversation
	idempotency   map[string]messageIdempotencyRecord
	readStates    map[string]int64
	now           func() time.Time
}

type memoryConversation struct {
	conversationID string
	chatType       string
	groupID        string
	participants   map[string]struct{}
	maxSeq         int64
	maxSeqTime     int64
	lastMessage    *Message
	messages       []Message
}

type messageIdempotencyRecord struct {
	payload        messageIdempotencyPayload
	conversationID string
	seq            int64
}

type messageIdempotencyPayload struct {
	senderID       string
	receiverID     string
	groupID        string
	chatType       string
	clientMsgID    string
	contentType    string
	content        string
	conversationID string
}

func NewMemoryMessageRepository() *MemoryMessageRepository {
	return &MemoryMessageRepository{
		conversations: make(map[string]*memoryConversation),
		idempotency:   make(map[string]messageIdempotencyRecord),
		readStates:    make(map[string]int64),
		now:           time.Now,
	}
}

func (r *MemoryMessageRepository) CreateMessageIdempotent(_ context.Context, input CreateMessageInput) (Message, bool, error) {
	conversationID, err := inputConversationID(input)
	if err != nil {
		return Message{}, false, err
	}
	payload := messageIdempotencyPayload{
		senderID:       input.SenderID,
		receiverID:     input.ReceiverID,
		groupID:        input.GroupID,
		chatType:       input.ChatType,
		clientMsgID:    input.ClientMsgID,
		contentType:    input.ContentType,
		content:        input.Content,
		conversationID: conversationID,
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	idempotencyKey := messageIdempotencyKey(input.SenderID, input.ClientMsgID)
	if existing, exists := r.idempotency[idempotencyKey]; exists {
		if existing.payload != payload {
			return Message{}, false, apperror.AlreadyExists("idempotency conflict")
		}
		message, err := r.messageBySeqLocked(existing.conversationID, existing.seq)
		if err != nil {
			return Message{}, false, err
		}
		return message, true, nil
	}

	conversation := r.ensureConversationLocked(conversationID, input)
	conversation.maxSeq++
	nowMillis := r.now().UTC().UnixMilli()

	r.nextMessageID++
	message := Message{
		ServerMsgID:    fmt.Sprintf("msg_%06d", r.nextMessageID),
		ClientMsgID:    input.ClientMsgID,
		ConversationID: conversationID,
		Seq:            conversation.maxSeq,
		SenderID:       input.SenderID,
		ReceiverID:     input.ReceiverID,
		GroupID:        input.GroupID,
		ChatType:       input.ChatType,
		ContentType:    input.ContentType,
		Content:        input.Content,
		SendTime:       nowMillis,
		CreatedAt:      nowMillis,
	}

	conversation.messages = append(conversation.messages, message.Clone())
	conversation.maxSeqTime = nowMillis
	lastMessage := message.Clone()
	conversation.lastMessage = &lastMessage
	r.idempotency[idempotencyKey] = messageIdempotencyRecord{
		payload:        payload,
		conversationID: conversationID,
		seq:            message.Seq,
	}
	r.setReadSeqLocked(input.SenderID, conversationID, message.Seq)

	return message.Clone(), false, nil
}

func (r *MemoryMessageRepository) GetMessages(_ context.Context, conversationID string, fromSeq, toSeq int64, limit int, order string) ([]Message, bool, int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	conversation, exists := r.conversations[conversationID]
	if !exists {
		return nil, false, 0, apperror.NotFound("conversation not found")
	}
	if fromSeq <= 0 {
		fromSeq = 1
	}
	if toSeq <= 0 || toSeq > conversation.maxSeq {
		toSeq = conversation.maxSeq
	}
	if limit <= 0 {
		limit = 50
	}
	if fromSeq > toSeq || conversation.maxSeq == 0 {
		return []Message{}, true, fromSeq, nil
	}

	messages := make([]Message, 0)
	for _, message := range conversation.messages {
		if message.Seq >= fromSeq && message.Seq <= toSeq {
			messages = append(messages, message.Clone())
		}
	}
	if order == "desc" {
		for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
			messages[i], messages[j] = messages[j], messages[i]
		}
	}

	isEnd := true
	if len(messages) > limit {
		isEnd = false
		messages = messages[:limit]
	}

	nextSeq := fromSeq
	if len(messages) > 0 {
		if order == "desc" {
			nextSeq = messages[len(messages)-1].Seq - 1
		} else {
			nextSeq = messages[len(messages)-1].Seq + 1
		}
	}
	return messages, isEnd, nextSeq, nil
}

func (r *MemoryMessageRepository) GetConversationSeqStates(_ context.Context, userID string, conversationIDs []string) ([]ConversationSeqState, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ids := conversationIDs
	if len(ids) == 0 {
		ids = make([]string, 0, len(r.conversations))
		for conversationID, conversation := range r.conversations {
			if _, ok := conversation.participants[userID]; ok {
				ids = append(ids, conversationID)
			}
		}
		sort.Strings(ids)
	}

	states := make([]ConversationSeqState, 0, len(ids))
	for _, conversationID := range ids {
		conversation, exists := r.conversations[conversationID]
		if !exists {
			return nil, apperror.NotFound("conversation not found")
		}
		if _, ok := conversation.participants[userID]; !ok {
			return nil, apperror.NotFound("conversation not found")
		}
		states = append(states, r.conversationSeqStateLocked(userID, conversation).Clone())
	}

	return states, nil
}

func (r *MemoryMessageRepository) SetUserHasReadSeqMax(_ context.Context, userID, conversationID string, seq int64) (ConversationSeqState, bool, error) {
	if seq < 0 {
		return ConversationSeqState{}, false, apperror.InvalidArgument("has_read_seq must be greater than or equal to 0")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	conversation, exists := r.conversations[conversationID]
	if !exists {
		return ConversationSeqState{}, false, apperror.NotFound("conversation not found")
	}
	if _, ok := conversation.participants[userID]; !ok {
		return ConversationSeqState{}, false, apperror.NotFound("conversation not found")
	}
	if seq > conversation.maxSeq {
		return ConversationSeqState{}, false, apperror.InvalidArgument("has_read_seq cannot exceed max_seq")
	}

	current := r.readStates[userConversationStateKey(userID, conversationID)]
	updated := false
	if seq > current {
		r.readStates[userConversationStateKey(userID, conversationID)] = seq
		updated = true
	}

	return r.conversationSeqStateLocked(userID, conversation).Clone(), updated, nil
}

func (r *MemoryMessageRepository) ensureConversationLocked(conversationID string, input CreateMessageInput) *memoryConversation {
	conversation, exists := r.conversations[conversationID]
	if !exists {
		conversation = &memoryConversation{
			conversationID: conversationID,
			chatType:       input.ChatType,
			groupID:        input.GroupID,
			participants:   make(map[string]struct{}),
		}
		r.conversations[conversationID] = conversation
	}

	for _, userID := range input.ParticipantUserIDs {
		if userID != "" {
			conversation.participants[userID] = struct{}{}
		}
	}
	conversation.participants[input.SenderID] = struct{}{}
	if input.ChatType == ChatTypeSingle && input.ReceiverID != "" {
		conversation.participants[input.ReceiverID] = struct{}{}
	}

	return conversation
}

func (r *MemoryMessageRepository) conversationSeqStateLocked(userID string, conversation *memoryConversation) ConversationSeqState {
	hasReadSeq := r.readStates[userConversationStateKey(userID, conversation.conversationID)]
	unreadCount := conversation.maxSeq - hasReadSeq
	if unreadCount < 0 {
		unreadCount = 0
	}

	state := ConversationSeqState{
		ConversationID: conversation.conversationID,
		MaxSeq:         conversation.maxSeq,
		HasReadSeq:     hasReadSeq,
		UnreadCount:    unreadCount,
		MaxSeqTime:     conversation.maxSeqTime,
	}
	if conversation.lastMessage != nil {
		lastMessage := conversation.lastMessage.Clone()
		state.LastMessage = &lastMessage
	}
	return state
}

func (r *MemoryMessageRepository) messageBySeqLocked(conversationID string, seq int64) (Message, error) {
	conversation, exists := r.conversations[conversationID]
	if !exists {
		return Message{}, apperror.NotFound("conversation not found")
	}
	if seq <= 0 || seq > int64(len(conversation.messages)) {
		return Message{}, apperror.NotFound("message not found")
	}
	return conversation.messages[seq-1].Clone(), nil
}

func (r *MemoryMessageRepository) setReadSeqLocked(userID, conversationID string, seq int64) {
	key := userConversationStateKey(userID, conversationID)
	if seq > r.readStates[key] {
		r.readStates[key] = seq
	}
}

func inputConversationID(input CreateMessageInput) (string, error) {
	switch input.ChatType {
	case ChatTypeSingle:
		return SingleConversationID(input.SenderID, input.ReceiverID), nil
	case ChatTypeGroup:
		return GroupConversationID(input.GroupID), nil
	default:
		return "", apperror.InvalidArgument("chat_type must be single or group")
	}
}

func messageIdempotencyKey(senderID string, clientMsgID string) string {
	return senderID + "\x00" + clientMsgID
}

func userConversationStateKey(userID string, conversationID string) string {
	return userID + "\x00" + conversationID
}
