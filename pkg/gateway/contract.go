package gateway

import "encoding/json"

const (
	CommandSendMessage          = "send_message"
	CommandPullMessages         = "pull_messages"
	CommandGetConversationSeqs  = "get_conversation_seqs"
	CommandMarkConversationRead = "mark_conversation_read"
	CommandHeartbeat            = "heartbeat"
)

const (
	RPCSendMessage            = "SendMessage"
	RPCPullMessages           = "PullMessages"
	RPCGetConversationSeqs    = "GetConversationSeqs"
	RPCMarkConversationAsRead = "MarkConversationAsRead"
)

const (
	AckStatusOK    = "ok"
	AckStatusError = "error"
)

type CommandEnvelope struct {
	RequestID string          `json:"requestId"`
	Command   string          `json:"command"`
	Payload   json.RawMessage `json:"payload,omitempty"`
}

type CommandACK struct {
	RequestID string        `json:"requestId"`
	Command   string        `json:"command,omitempty"`
	Status    string        `json:"status"`
	Error     *CommandError `json:"error,omitempty"`
	Payload   interface{}   `json:"payload,omitempty"`
}

type CommandError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type CommandRPCMapping struct {
	Command        string
	RPC            string
	RequestFields  []FieldMapping
	ResponseFields []FieldMapping
}

type FieldMapping struct {
	CommandField string
	RPCField     string
}

type SendMessageCommandRequest struct {
	ChatType    string `json:"chatType"`
	ReceiverID  string `json:"receiverId,omitempty"`
	GroupID     string `json:"groupId,omitempty"`
	ClientMsgID string `json:"clientMsgId"`
	ContentType string `json:"contentType"`
	Content     string `json:"content"`
}

type SendMessageRPCRequest struct {
	SenderID    string
	ReceiverID  string
	GroupID     string
	ChatType    string
	ClientMsgID string
	ContentType string
	Content     string
}

type SendMessageRPCResponse struct {
	Message      MessageSnapshot
	Deduplicated bool
}

type SendMessageCommandResponse struct {
	Message      MessageSnapshot `json:"message"`
	Deduplicated bool            `json:"deduplicated"`
}

type PullMessagesCommandRequest struct {
	ConversationID string `json:"conversationId"`
	FromSeq        int64  `json:"fromSeq"`
	ToSeq          int64  `json:"toSeq"`
	Limit          int32  `json:"limit"`
	Order          string `json:"order,omitempty"`
}

type PullMessagesRPCRequest struct {
	UserID         string
	ConversationID string
	FromSeq        int64
	ToSeq          int64
	Limit          int32
	Order          string
}

type PullMessagesRPCResponse struct {
	Messages []MessageSnapshot
	IsEnd    bool
	NextSeq  int64
}

type PullMessagesCommandResponse struct {
	Messages []MessageSnapshot `json:"messages"`
	IsEnd    bool              `json:"isEnd"`
	NextSeq  int64             `json:"nextSeq"`
}

type GetConversationSeqsCommandRequest struct {
	ConversationIDs []string `json:"conversationIds,omitempty"`
}

type GetConversationSeqsRPCRequest struct {
	UserID          string
	ConversationIDs []string
}

type GetConversationSeqsRPCResponse struct {
	States []ConversationSeqState
}

type GetConversationSeqsCommandResponse struct {
	States []ConversationSeqState `json:"states"`
}

type MarkConversationReadCommandRequest struct {
	ConversationID string `json:"conversationId"`
	HasReadSeq     int64  `json:"hasReadSeq"`
}

type MarkConversationAsReadRPCRequest struct {
	UserID         string
	ConversationID string
	HasReadSeq     int64
}

type MarkConversationAsReadRPCResponse struct {
	ConversationID string
	HasReadSeq     int64
	MaxSeq         int64
	UnreadCount    int64
	Updated        bool
}

type MarkConversationReadCommandResponse struct {
	ConversationID string `json:"conversationId"`
	HasReadSeq     int64  `json:"hasReadSeq"`
	MaxSeq         int64  `json:"maxSeq"`
	UnreadCount    int64  `json:"unreadCount"`
	Updated        bool   `json:"updated"`
}

type MessageSnapshot struct {
	ServerMsgID           string `json:"serverMsgId"`
	ClientMsgID           string `json:"clientMsgId"`
	ConversationID        string `json:"conversationId"`
	Seq                   int64  `json:"seq"`
	SenderID              string `json:"senderId"`
	ReceiverID            string `json:"receiverId,omitempty"`
	GroupID               string `json:"groupId,omitempty"`
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

type ConversationSeqState struct {
	ConversationID string           `json:"conversationId"`
	MaxSeq         int64            `json:"maxSeq"`
	HasReadSeq     int64            `json:"hasReadSeq"`
	UnreadCount    int64            `json:"unreadCount"`
	MaxSeqTime     int64            `json:"maxSeqTime"`
	LastMessage    *MessageSnapshot `json:"lastMessage,omitempty"`
}

func MapSendMessageRequest(userID string, req SendMessageCommandRequest) SendMessageRPCRequest {
	return SendMessageRPCRequest{
		SenderID:    userID,
		ReceiverID:  req.ReceiverID,
		GroupID:     req.GroupID,
		ChatType:    req.ChatType,
		ClientMsgID: req.ClientMsgID,
		ContentType: req.ContentType,
		Content:     req.Content,
	}
}

func MapSendMessageResponse(resp SendMessageRPCResponse) SendMessageCommandResponse {
	return SendMessageCommandResponse{
		Message:      resp.Message,
		Deduplicated: resp.Deduplicated,
	}
}

func MapPullMessagesRequest(userID string, req PullMessagesCommandRequest) PullMessagesRPCRequest {
	return PullMessagesRPCRequest{
		UserID:         userID,
		ConversationID: req.ConversationID,
		FromSeq:        req.FromSeq,
		ToSeq:          req.ToSeq,
		Limit:          req.Limit,
		Order:          req.Order,
	}
}

func MapPullMessagesResponse(resp PullMessagesRPCResponse) PullMessagesCommandResponse {
	return PullMessagesCommandResponse{
		Messages: resp.Messages,
		IsEnd:    resp.IsEnd,
		NextSeq:  resp.NextSeq,
	}
}

func MapGetConversationSeqsRequest(userID string, req GetConversationSeqsCommandRequest) GetConversationSeqsRPCRequest {
	return GetConversationSeqsRPCRequest{
		UserID:          userID,
		ConversationIDs: append([]string(nil), req.ConversationIDs...),
	}
}

func MapGetConversationSeqsResponse(resp GetConversationSeqsRPCResponse) GetConversationSeqsCommandResponse {
	return GetConversationSeqsCommandResponse{
		States: append([]ConversationSeqState(nil), resp.States...),
	}
}

func MapMarkConversationReadRequest(userID string, req MarkConversationReadCommandRequest) MarkConversationAsReadRPCRequest {
	return MarkConversationAsReadRPCRequest{
		UserID:         userID,
		ConversationID: req.ConversationID,
		HasReadSeq:     req.HasReadSeq,
	}
}

func MapMarkConversationReadResponse(resp MarkConversationAsReadRPCResponse) MarkConversationReadCommandResponse {
	return MarkConversationReadCommandResponse{
		ConversationID: resp.ConversationID,
		HasReadSeq:     resp.HasReadSeq,
		MaxSeq:         resp.MaxSeq,
		UnreadCount:    resp.UnreadCount,
		Updated:        resp.Updated,
	}
}
