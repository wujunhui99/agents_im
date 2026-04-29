package repository

import (
	"context"
	"errors"
	"time"
)

const (
	MessageStorageChatTypeSingle = "single"
	MessageStorageChatTypeGroup  = "group"

	MessageStorageOrderAsc  = "asc"
	MessageStorageOrderDesc = "desc"
)

var (
	ErrMessageStorageIdempotencyConflict  = errors.New("message storage: idempotency conflict")
	ErrMessageStorageConversationNotFound = errors.New("message storage: conversation not found")
	ErrMessageStorageInvalidSeqRange      = errors.New("message storage: invalid seq range")
	ErrMessageStorageReadSeqBeyondMax     = errors.New("message storage: read seq greater than max seq")
)

// MessageStorageRepository defines durable message storage behavior without
// depending on message API, proto, or handler packages.
type MessageStorageRepository interface {
	CreateMessageIdempotent(ctx context.Context, input CreateStoredMessageInput) (StoredMessage, bool, error)
	PullMessages(ctx context.Context, query MessagePullQuery) (MessagePullResult, error)
	GetConversationSeqStates(ctx context.Context, userID string, conversationIDs []string) ([]MessageStorageConversationSeqState, error)
	SetUserHasReadSeqMax(ctx context.Context, userID string, conversationID string, seq int64) (MessageStorageConversationSeqState, bool, error)
}

type CreateStoredMessageInput struct {
	ServerMsgID    string
	ClientMsgID    string
	SenderID       string
	ConversationID string
	ChatType       string
	ReceiverID     string
	GroupID        string
	ContentType    string
	Content        []byte
	PayloadHash    string
	VisibleUserIDs []string
	SendTime       time.Time
}

type StoredMessage struct {
	ServerMsgID    string
	ClientMsgID    string
	SenderID       string
	ConversationID string
	Seq            int64
	ChatType       string
	ReceiverID     string
	GroupID        string
	ContentType    string
	Content        []byte
	PayloadHash    string
	SendTime       time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type MessagePullQuery struct {
	UserID         string
	ConversationID string
	FromSeq        int64
	ToSeq          int64
	Limit          int
	Order          string
}

type MessagePullResult struct {
	Messages []StoredMessage
	IsEnd    bool
	NextSeq  int64
}

type MessageStorageConversationSeqState struct {
	UserID         string
	ConversationID string
	MaxSeq         int64
	HasReadSeq     int64
	UnreadCount    int64
	MaxSeqTime     time.Time
	LastMessage    *StoredMessage
	UpdatedAt      time.Time
}

func MessageStorageSingleConversationID(userA string, userB string) string {
	lower, higher := MessageStorageOrderedSingleUsers(userA, userB)
	return "single:" + lower + ":" + higher
}

func MessageStorageOrderedSingleUsers(userA string, userB string) (string, string) {
	if userA <= userB {
		return userA, userB
	}
	return userB, userA
}

func MessageStorageGroupConversationID(groupID string) string {
	return "group:" + groupID
}

func MessageStorageUnreadCount(maxSeq int64, hasReadSeq int64) int64 {
	if maxSeq <= hasReadSeq {
		return 0
	}
	return maxSeq - hasReadSeq
}

func MessageStorageAdvanceReadSeq(current int64, candidate int64) int64 {
	if candidate > current {
		return candidate
	}
	return current
}
