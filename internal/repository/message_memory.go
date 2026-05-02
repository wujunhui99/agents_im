package repository

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/wujunhui99/agents_im/internal/apperror"
)

type MemoryMessageRepository struct {
	mu               sync.RWMutex
	nextMessageID    uint64
	nextOutboxID     uint64
	conversations    map[string]*memoryConversation
	idempotency      map[string]messageIdempotencyRecord
	readStates       map[string]int64
	visibleStates    map[string]int64
	outbox           []OutboxEvent
	deliveryAttempts map[string]DeliveryAttempt
	now              func() time.Time
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
	senderID              string
	receiverID            string
	groupID               string
	chatType              string
	clientMsgID           string
	contentType           string
	content               string
	messageOrigin         string
	agentAccountID        string
	triggerServerMsgID    string
	agentRunID            string
	allowRecursiveTrigger bool
	conversationID        string
}

func NewMemoryMessageRepository() *MemoryMessageRepository {
	return &MemoryMessageRepository{
		conversations:    make(map[string]*memoryConversation),
		idempotency:      make(map[string]messageIdempotencyRecord),
		readStates:       make(map[string]int64),
		visibleStates:    make(map[string]int64),
		deliveryAttempts: make(map[string]DeliveryAttempt),
		now:              time.Now,
	}
}

func (r *MemoryMessageRepository) CreateMessageIdempotent(_ context.Context, input CreateMessageInput) (Message, bool, error) {
	if _, err := normalizeMessageOriginInput(&input); err != nil {
		return Message{}, false, err
	}
	conversationID, err := validateCreateMessageInput(input)
	if err != nil {
		return Message{}, false, err
	}
	payload := messageIdempotencyPayload{
		senderID:              input.SenderID,
		receiverID:            input.ReceiverID,
		groupID:               input.GroupID,
		chatType:              input.ChatType,
		clientMsgID:           input.ClientMsgID,
		contentType:           input.ContentType,
		content:               input.Content,
		messageOrigin:         input.MessageOrigin,
		agentAccountID:        input.AgentAccountID,
		triggerServerMsgID:    input.TriggerServerMsgID,
		agentRunID:            input.AgentRunID,
		allowRecursiveTrigger: input.AllowRecursiveTrigger,
		conversationID:        conversationID,
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
	now := r.now().UTC()
	nowMillis := now.UnixMilli()

	r.nextMessageID++
	message := Message{
		ServerMsgID:           fmt.Sprintf("msg_%06d", r.nextMessageID),
		ClientMsgID:           input.ClientMsgID,
		ConversationID:        conversationID,
		Seq:                   conversation.maxSeq,
		SenderID:              input.SenderID,
		ReceiverID:            input.ReceiverID,
		GroupID:               input.GroupID,
		ChatType:              input.ChatType,
		ContentType:           input.ContentType,
		Content:               input.Content,
		MessageOrigin:         input.MessageOrigin,
		AgentAccountID:        input.AgentAccountID,
		TriggerServerMsgID:    input.TriggerServerMsgID,
		AgentRunID:            input.AgentRunID,
		AllowRecursiveTrigger: input.AllowRecursiveTrigger,
		SendTime:              nowMillis,
		CreatedAt:             nowMillis,
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
	for _, userID := range visibleUserIDs(input) {
		r.setVisibleSeqLocked(userID, conversationID, message.Seq)
	}
	r.setReadSeqLocked(input.SenderID, conversationID, message.Seq)
	if err := r.appendMessageCreatedOutboxLocked(message, input, nowMillis); err != nil {
		return Message{}, false, err
	}
	if err := r.createDeliveryAttemptsAcceptedLocked(deliveryAttemptsForMessage(message, input), now); err != nil {
		return Message{}, false, err
	}

	return message.Clone(), false, nil
}

func (r *MemoryMessageRepository) PollPending(_ context.Context, workerID string, limit int, lockDuration time.Duration) ([]OutboxEvent, error) {
	workerID = strings.TrimSpace(workerID)
	if workerID == "" {
		return nil, apperror.InvalidArgument("worker_id is required")
	}
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	if lockDuration <= 0 {
		lockDuration = 30 * time.Second
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	now := r.now().UTC()
	lockedUntil := now.Add(lockDuration)
	events := make([]OutboxEvent, 0, limit)
	for i := range r.outbox {
		if len(events) >= limit {
			break
		}
		event := &r.outbox[i]
		if event.Status != OutboxStatusPending {
			continue
		}
		if event.NextAttemptAt.After(now) {
			continue
		}
		if !event.LockedUntil.IsZero() && event.LockedUntil.After(now) {
			continue
		}
		event.LockedBy = workerID
		event.LockedUntil = lockedUntil
		event.UpdatedAt = now
		events = append(events, event.Clone())
	}
	return events, nil
}

func (r *MemoryMessageRepository) MarkPublished(_ context.Context, eventID string, workerID string) error {
	workerID = strings.TrimSpace(workerID)
	if workerID == "" {
		return apperror.InvalidArgument("worker_id is required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	now := r.now().UTC()
	event, err := r.lockedOutboxEventLocked(eventID, workerID, now)
	if err != nil {
		return err
	}
	event.Status = OutboxStatusPublished
	event.LockedBy = ""
	event.LockedUntil = time.Time{}
	event.PublishedAt = now
	event.UpdatedAt = now
	r.markDeliveryAttemptsPublishedLocked(event.ServerMsgID, nil, now)
	return nil
}

func (r *MemoryMessageRepository) MarkFailed(_ context.Context, eventID string, workerID string, failure OutboxFailure) error {
	workerID = strings.TrimSpace(workerID)
	if workerID == "" {
		return apperror.InvalidArgument("worker_id is required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	now := r.now().UTC()
	event, err := r.lockedOutboxEventLocked(eventID, workerID, now)
	if err != nil {
		return err
	}
	event.AttemptCount++
	event.LastError = strings.TrimSpace(failure.LastError)
	event.LockedBy = ""
	event.LockedUntil = time.Time{}
	event.UpdatedAt = now
	if failure.NextAttemptAt.IsZero() {
		event.Status = OutboxStatusFailed
		event.NextAttemptAt = now
		return nil
	}
	event.Status = OutboxStatusPending
	event.NextAttemptAt = failure.NextAttemptAt.UTC()
	return nil
}

func (r *MemoryMessageRepository) GetMessages(_ context.Context, conversationID string, fromSeq, toSeq int64, limit int, order string) ([]Message, bool, int64, error) {
	var err error
	fromSeq, toSeq, limit, order, err = normalizeMessagePullRange(fromSeq, toSeq, limit, order)
	if err != nil {
		return nil, false, 0, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	conversation, exists := r.conversations[conversationID]
	if !exists {
		return nil, false, 0, apperror.NotFound("conversation not found")
	}
	if toSeq <= 0 || toSeq > conversation.maxSeq {
		toSeq = conversation.maxSeq
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
		prefix := userID + "\x00"
		for key := range r.visibleStates {
			if strings.HasPrefix(key, prefix) {
				ids = append(ids, strings.TrimPrefix(key, prefix))
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
		visibleSeq, ok := r.visibleSeqLocked(userID, conversationID)
		if !ok {
			return nil, apperror.NotFound("conversation not found")
		}
		states = append(states, r.conversationSeqStateLocked(userID, conversation, visibleSeq).Clone())
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
	visibleSeq, ok := r.visibleSeqLocked(userID, conversationID)
	if !ok {
		return ConversationSeqState{}, false, apperror.NotFound("conversation not found")
	}
	if seq > visibleSeq {
		return ConversationSeqState{}, false, apperror.InvalidArgument("has_read_seq cannot exceed max_seq")
	}

	current := r.readStates[userConversationStateKey(userID, conversationID)]
	updated := false
	if seq > current {
		r.readStates[userConversationStateKey(userID, conversationID)] = seq
		updated = true
	}

	return r.conversationSeqStateLocked(userID, conversation, visibleSeq).Clone(), updated, nil
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

func (r *MemoryMessageRepository) conversationSeqStateLocked(userID string, conversation *memoryConversation, visibleSeq int64) ConversationSeqState {
	hasReadSeq := r.readStates[userConversationStateKey(userID, conversation.conversationID)]
	unreadCount := visibleSeq - hasReadSeq
	if unreadCount < 0 {
		unreadCount = 0
	}

	state := ConversationSeqState{
		ConversationID: conversation.conversationID,
		MaxSeq:         visibleSeq,
		HasReadSeq:     hasReadSeq,
		UnreadCount:    unreadCount,
	}
	if visibleSeq > 0 && visibleSeq <= int64(len(conversation.messages)) {
		lastMessage := conversation.messages[visibleSeq-1].Clone()
		state.MaxSeqTime = lastMessage.SendTime
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

func (r *MemoryMessageRepository) setVisibleSeqLocked(userID, conversationID string, seq int64) {
	key := userConversationStateKey(userID, conversationID)
	if seq > r.visibleStates[key] {
		r.visibleStates[key] = seq
	}
}

func (r *MemoryMessageRepository) visibleSeqLocked(userID, conversationID string) (int64, bool) {
	seq, ok := r.visibleStates[userConversationStateKey(userID, conversationID)]
	return seq, ok
}

func (r *MemoryMessageRepository) appendMessageCreatedOutboxLocked(message Message, input CreateMessageInput, nowMillis int64) error {
	payload, err := messageCreatedOutboxPayload(message, input)
	if err != nil {
		return err
	}
	now := time.UnixMilli(nowMillis).UTC()
	r.nextOutboxID++
	r.outbox = append(r.outbox, OutboxEvent{
		EventID:        fmt.Sprintf("outbox_%06d", r.nextOutboxID),
		EventType:      OutboxEventTypeMessageCreated,
		AggregateType:  OutboxAggregateTypeMessage,
		AggregateID:    message.ServerMsgID,
		ConversationID: message.ConversationID,
		ServerMsgID:    message.ServerMsgID,
		Seq:            message.Seq,
		Payload:        payload,
		Status:         OutboxStatusPending,
		NextAttemptAt:  now,
		CreatedAt:      now,
		UpdatedAt:      now,
	})
	return nil
}

func (r *MemoryMessageRepository) lockedOutboxEventLocked(eventID string, workerID string, now time.Time) (*OutboxEvent, error) {
	eventID = strings.TrimSpace(eventID)
	if eventID == "" {
		return nil, apperror.InvalidArgument("event_id is required")
	}
	for i := range r.outbox {
		event := &r.outbox[i]
		if event.EventID != eventID {
			continue
		}
		if event.Status != OutboxStatusPending || event.LockedBy != workerID || event.LockedUntil.IsZero() || !event.LockedUntil.After(now) {
			return nil, apperror.NotFound("outbox event lock not found")
		}
		return event, nil
	}
	return nil, apperror.NotFound("outbox event not found")
}

func inputConversationID(input CreateMessageInput) (string, error) {
	switch input.ChatType {
	case ChatTypeSingle:
		if err := validateMessageConversationComponentID(input.SenderID, "sender_id"); err != nil {
			return "", err
		}
		if err := validateMessageConversationComponentID(input.ReceiverID, "receiver_id"); err != nil {
			return "", err
		}
		if input.SenderID == input.ReceiverID {
			return "", apperror.InvalidArgument("sender_id and receiver_id must be different")
		}
		if strings.TrimSpace(input.GroupID) != "" {
			return "", apperror.InvalidArgument("group_id must be empty for single chat")
		}
		conversationID := SingleConversationID(input.SenderID, input.ReceiverID)
		if err := validateMessageConversationID(conversationID); err != nil {
			return "", err
		}
		return conversationID, nil
	case ChatTypeGroup:
		if err := validateMessageConversationComponentID(input.GroupID, "group_id"); err != nil {
			return "", err
		}
		if strings.TrimSpace(input.ReceiverID) != "" {
			return "", apperror.InvalidArgument("receiver_id must be empty for group chat")
		}
		conversationID := GroupConversationID(input.GroupID)
		if err := validateMessageConversationID(conversationID); err != nil {
			return "", err
		}
		return conversationID, nil
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
