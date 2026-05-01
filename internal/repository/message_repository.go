package repository

import (
	"context"
	"sort"
)

const (
	ChatTypeSingle = "single"
	ChatTypeGroup  = "group"

	ContentTypeText  = "text"
	ContentTypeImage = "image"
	ContentTypeFile  = "file"
)

type Message struct {
	ServerMsgID    string `json:"serverMsgId"`
	ClientMsgID    string `json:"clientMsgId"`
	ConversationID string `json:"conversationId"`
	Seq            int64  `json:"seq"`
	SenderID       string `json:"senderId"`
	ReceiverID     string `json:"receiverId"`
	GroupID        string `json:"groupId"`
	ChatType       string `json:"chatType"`
	ContentType    string `json:"contentType"`
	Content        string `json:"content"`
	SendTime       int64  `json:"sendTime"`
	CreatedAt      int64  `json:"createdAt"`
}

func (m Message) Clone() Message {
	return m
}

type ConversationSeqState struct {
	ConversationID string   `json:"conversationId"`
	MaxSeq         int64    `json:"maxSeq"`
	HasReadSeq     int64    `json:"hasReadSeq"`
	UnreadCount    int64    `json:"unreadCount"`
	MaxSeqTime     int64    `json:"maxSeqTime"`
	LastMessage    *Message `json:"lastMessage,omitempty"`
}

func (s ConversationSeqState) Clone() ConversationSeqState {
	if s.LastMessage != nil {
		lastMessage := s.LastMessage.Clone()
		s.LastMessage = &lastMessage
	}
	return s
}

type CreateMessageInput struct {
	SenderID           string
	ReceiverID         string
	GroupID            string
	ChatType           string
	ClientMsgID        string
	ContentType        string
	Content            string
	ParticipantUserIDs []string
}

type MessageRepository interface {
	CreateMessageIdempotent(ctx context.Context, input CreateMessageInput) (Message, bool, error)
	GetMessages(ctx context.Context, conversationID string, fromSeq, toSeq int64, limit int, order string) ([]Message, bool, int64, error)
	GetConversationSeqStates(ctx context.Context, userID string, conversationIDs []string) ([]ConversationSeqState, error)
	SetUserHasReadSeqMax(ctx context.Context, userID, conversationID string, seq int64) (ConversationSeqState, bool, error)
}

func SingleConversationID(userA string, userB string) string {
	users := []string{userA, userB}
	sort.Strings(users)
	return "single:" + users[0] + ":" + users[1]
}

func GroupConversationID(groupID string) string {
	return "group:" + groupID
}
