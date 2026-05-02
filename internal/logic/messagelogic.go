package logic

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/wujunhui99/agents_im/internal/apperror"
	"github.com/wujunhui99/agents_im/internal/observability"
	"github.com/wujunhui99/agents_im/internal/repository"
)

const (
	MessageChatTypeSingle = repository.ChatTypeSingle
	MessageChatTypeGroup  = repository.ChatTypeGroup

	MessageContentTypeText  = repository.ContentTypeText
	MessageContentTypeImage = repository.ContentTypeImage
	MessageContentTypeFile  = repository.ContentTypeFile

	MessageOriginHuman  = repository.MessageOriginHuman
	MessageOriginAI     = repository.MessageOriginAI
	MessageOriginSystem = repository.MessageOriginSystem
)

type GroupMemberLister interface {
	ListMembers(ctx context.Context, req ListMembersRequest) (ListMembersResponse, error)
}

type MessageCreatedHook interface {
	OnMessageCreated(ctx context.Context, input MessageCreatedHookInput) error
}

type MessageCreatedHookFunc func(ctx context.Context, input MessageCreatedHookInput) error

func (f MessageCreatedHookFunc) OnMessageCreated(ctx context.Context, input MessageCreatedHookInput) error {
	if f == nil {
		return nil
	}
	return f(ctx, input)
}

type MessageCreatedHookInput struct {
	EventID          string
	OperationID      string
	TraceID          string
	Message          Message
	Deduplicated     bool
	RecipientUserIDs []string
}

type MessageLogic struct {
	repo               repository.MessageRepository
	userExists         UserExistenceChecker
	groups             GroupMemberLister
	media              MessageMediaValidator
	messageCreatedHook MessageCreatedHook
}

type MessageMediaValidator interface {
	ValidateMessageMedia(ctx context.Context, ownerUserID string, contentType string, content string) error
}

func NewMessageLogic(repo repository.MessageRepository) *MessageLogic {
	return &MessageLogic{repo: repo}
}

func NewMessageLogicWithValidators(repo repository.MessageRepository, userExists UserExistenceChecker, groups GroupMemberLister) *MessageLogic {
	return &MessageLogic{repo: repo, userExists: userExists, groups: groups}
}

func NewMessageLogicWithMediaValidator(repo repository.MessageRepository, userExists UserExistenceChecker, groups GroupMemberLister, media MessageMediaValidator) *MessageLogic {
	return &MessageLogic{repo: repo, userExists: userExists, groups: groups, media: media}
}

func (l *MessageLogic) WithMediaValidator(media MessageMediaValidator) *MessageLogic {
	if l != nil {
		l.media = media
	}
	return l
}

func (l *MessageLogic) SetMessageCreatedHook(hook MessageCreatedHook) {
	if l == nil {
		return
	}
	l.messageCreatedHook = hook
}

type Message = repository.Message

type ConversationSeqState = repository.ConversationSeqState

type SendMessageRequest struct {
	SenderID              string `json:"senderId"`
	ReceiverID            string `json:"receiverId"`
	GroupID               string `json:"groupId"`
	ChatType              string `json:"chatType"`
	ClientMsgID           string `json:"clientMsgId"`
	ContentType           string `json:"contentType"`
	Content               string `json:"content"`
	MessageOrigin         string `json:"messageOrigin,omitempty"`
	AgentAccountID        string `json:"agentAccountId,omitempty"`
	TriggerServerMsgID    string `json:"triggerServerMsgId,omitempty"`
	AgentRunID            string `json:"agentRunId,omitempty"`
	AllowRecursiveTrigger bool   `json:"allowRecursiveTrigger,omitempty"`
}

type SendMessageResponse struct {
	Message          Message  `json:"message"`
	Deduplicated     bool     `json:"deduplicated"`
	RecipientUserIDs []string `json:"-"`
}

type PullMessagesRequest struct {
	UserID         string `json:"userId"`
	ConversationID string `json:"conversationId"`
	FromSeq        int64  `json:"fromSeq"`
	ToSeq          int64  `json:"toSeq"`
	Limit          int    `json:"limit"`
	Order          string `json:"order"`
}

type PullMessagesResponse struct {
	Messages []Message `json:"messages"`
	IsEnd    bool      `json:"isEnd"`
	NextSeq  int64     `json:"nextSeq"`
}

type GetConversationSeqsRequest struct {
	UserID          string   `json:"userId"`
	ConversationIDs []string `json:"conversationIds"`
}

type GetConversationSeqsResponse struct {
	States []ConversationSeqState `json:"states"`
}

type MarkConversationAsReadRequest struct {
	UserID         string `json:"userId"`
	ConversationID string `json:"conversationId"`
	HasReadSeq     int64  `json:"hasReadSeq"`
}

type MarkConversationAsReadResponse struct {
	ConversationID string `json:"conversationId"`
	HasReadSeq     int64  `json:"hasReadSeq"`
	MaxSeq         int64  `json:"maxSeq"`
	UnreadCount    int64  `json:"unreadCount"`
	Updated        bool   `json:"updated"`
}

func (l *MessageLogic) SendMessage(ctx context.Context, req SendMessageRequest) (SendMessageResponse, error) {
	metricsStatus := "accepted"
	metricsChatType := strings.ToLower(strings.TrimSpace(req.ChatType))
	defer func() {
		observability.RecordMessageSend(metricsStatus, metricsChatType)
	}()

	if l.repo == nil {
		metricsStatus = "failed"
		return SendMessageResponse{}, apperror.Internal("message repository is not configured")
	}

	input, err := l.normalizeSendMessage(ctx, req)
	if err != nil {
		metricsStatus = "failed"
		return SendMessageResponse{}, err
	}
	metricsChatType = input.ChatType

	message, deduplicated, err := l.repo.CreateMessageIdempotent(ctx, input)
	if err != nil {
		metricsStatus = "failed"
		return SendMessageResponse{}, err
	}
	if deduplicated {
		metricsStatus = "deduplicated"
	}

	recipientUserIDs := repository.DeliveryRecipientUserIDs(input)
	if l.messageCreatedHook != nil {
		if err := l.messageCreatedHook.OnMessageCreated(ctx, MessageCreatedHookInput{
			EventID:          messageCreatedHookEventID(message),
			Message:          message.Clone(),
			Deduplicated:     deduplicated,
			RecipientUserIDs: append([]string(nil), recipientUserIDs...),
		}); err != nil {
			metricsStatus = "failed"
			return SendMessageResponse{}, err
		}
	}

	return SendMessageResponse{
		Message:          message,
		Deduplicated:     deduplicated,
		RecipientUserIDs: recipientUserIDs,
	}, nil
}

func (l *MessageLogic) PullMessages(ctx context.Context, req PullMessagesRequest) (PullMessagesResponse, error) {
	if l.repo == nil {
		return PullMessagesResponse{}, apperror.Internal("message repository is not configured")
	}

	userID, err := normalizeMessageRequiredID(req.UserID, "user_id")
	if err != nil {
		return PullMessagesResponse{}, err
	}
	conversationID, err := normalizeConversationID(req.ConversationID)
	if err != nil {
		return PullMessagesResponse{}, err
	}
	fromSeq, toSeq, limit, order, err := normalizePullRange(req.FromSeq, req.ToSeq, req.Limit, req.Order)
	if err != nil {
		return PullMessagesResponse{}, err
	}

	states, err := l.repo.GetConversationSeqStates(ctx, userID, []string{conversationID})
	if err != nil {
		return PullMessagesResponse{}, err
	}
	if len(states) == 1 {
		if toSeq == 0 || toSeq > states[0].MaxSeq {
			toSeq = states[0].MaxSeq
		}
	}

	messages, isEnd, nextSeq, err := l.repo.GetMessages(ctx, conversationID, fromSeq, toSeq, limit, order)
	if err != nil {
		return PullMessagesResponse{}, err
	}
	return PullMessagesResponse{Messages: messages, IsEnd: isEnd, NextSeq: nextSeq}, nil
}

func (l *MessageLogic) GetConversationSeqs(ctx context.Context, req GetConversationSeqsRequest) (GetConversationSeqsResponse, error) {
	if l.repo == nil {
		return GetConversationSeqsResponse{}, apperror.Internal("message repository is not configured")
	}

	userID, err := normalizeMessageRequiredID(req.UserID, "user_id")
	if err != nil {
		return GetConversationSeqsResponse{}, err
	}

	conversationIDs := make([]string, 0, len(req.ConversationIDs))
	for _, conversationID := range req.ConversationIDs {
		normalized, err := normalizeConversationID(conversationID)
		if err != nil {
			return GetConversationSeqsResponse{}, err
		}
		conversationIDs = append(conversationIDs, normalized)
	}

	states, err := l.repo.GetConversationSeqStates(ctx, userID, conversationIDs)
	if err != nil {
		return GetConversationSeqsResponse{}, err
	}
	return GetConversationSeqsResponse{States: states}, nil
}

func (l *MessageLogic) MarkConversationAsRead(ctx context.Context, req MarkConversationAsReadRequest) (MarkConversationAsReadResponse, error) {
	if l.repo == nil {
		return MarkConversationAsReadResponse{}, apperror.Internal("message repository is not configured")
	}

	userID, err := normalizeMessageRequiredID(req.UserID, "user_id")
	if err != nil {
		return MarkConversationAsReadResponse{}, err
	}
	conversationID, err := normalizeConversationID(req.ConversationID)
	if err != nil {
		return MarkConversationAsReadResponse{}, err
	}
	if req.HasReadSeq < 0 {
		return MarkConversationAsReadResponse{}, apperror.InvalidArgument("has_read_seq must be greater than or equal to 0")
	}

	state, updated, err := l.repo.SetUserHasReadSeqMax(ctx, userID, conversationID, req.HasReadSeq)
	if err != nil {
		return MarkConversationAsReadResponse{}, err
	}
	return MarkConversationAsReadResponse{
		ConversationID: state.ConversationID,
		HasReadSeq:     state.HasReadSeq,
		MaxSeq:         state.MaxSeq,
		UnreadCount:    state.UnreadCount,
		Updated:        updated,
	}, nil
}

func (l *MessageLogic) normalizeSendMessage(ctx context.Context, req SendMessageRequest) (repository.CreateMessageInput, error) {
	senderID, err := normalizeMessageConversationComponentID(req.SenderID, "sender_id")
	if err != nil {
		return repository.CreateMessageInput{}, err
	}
	chatType := strings.ToLower(strings.TrimSpace(req.ChatType))
	clientMsgID, err := normalizeMessageRequiredID(req.ClientMsgID, "client_msg_id")
	if err != nil {
		return repository.CreateMessageInput{}, err
	}
	contentType, content, err := l.normalizeMessageContent(ctx, senderID, req.ContentType, req.Content)
	if err != nil {
		return repository.CreateMessageInput{}, err
	}

	input := repository.CreateMessageInput{
		SenderID:    senderID,
		ChatType:    chatType,
		ClientMsgID: clientMsgID,
		ContentType: contentType,
		Content:     content,
	}
	if err := l.applyMessageOriginMetadata(&input, req); err != nil {
		return repository.CreateMessageInput{}, err
	}

	switch chatType {
	case MessageChatTypeSingle:
		receiverID, err := normalizeMessageConversationComponentID(req.ReceiverID, "receiver_id")
		if err != nil {
			return repository.CreateMessageInput{}, err
		}
		if senderID == receiverID {
			return repository.CreateMessageInput{}, apperror.InvalidArgument("sender_id and receiver_id must be different")
		}
		if strings.TrimSpace(req.GroupID) != "" {
			return repository.CreateMessageInput{}, apperror.InvalidArgument("group_id must be empty for single chat")
		}
		if err := l.ensureUserExists(ctx, senderID); err != nil {
			return repository.CreateMessageInput{}, err
		}
		if err := l.ensureUserExists(ctx, receiverID); err != nil {
			return repository.CreateMessageInput{}, err
		}
		input.ReceiverID = receiverID
		input.ParticipantUserIDs = []string{senderID, receiverID}
	case MessageChatTypeGroup:
		groupID, err := normalizeMessageConversationComponentID(req.GroupID, "group_id")
		if err != nil {
			return repository.CreateMessageInput{}, err
		}
		if strings.TrimSpace(req.ReceiverID) != "" {
			return repository.CreateMessageInput{}, apperror.InvalidArgument("receiver_id must be empty for group chat")
		}
		participantIDs, err := l.resolveGroupParticipants(ctx, groupID, senderID)
		if err != nil {
			return repository.CreateMessageInput{}, err
		}
		input.GroupID = groupID
		input.ParticipantUserIDs = participantIDs
	default:
		return repository.CreateMessageInput{}, apperror.InvalidArgument("chat_type must be single or group")
	}

	return input, nil
}

func (l *MessageLogic) normalizeMessageContent(ctx context.Context, senderID string, rawContentType string, rawContent string) (string, string, error) {
	contentType := strings.ToLower(strings.TrimSpace(rawContentType))
	switch contentType {
	case MessageContentTypeText:
		content := strings.TrimSpace(rawContent)
		if content == "" {
			return "", "", apperror.InvalidArgument("content is required")
		}
		if len([]rune(content)) > 4096 {
			return "", "", apperror.InvalidArgument("content must be 4096 characters or fewer")
		}
		return contentType, content, nil
	case MessageContentTypeImage, MessageContentTypeFile:
		content := strings.TrimSpace(rawContent)
		if content == "" {
			return "", "", apperror.InvalidArgument("content is required")
		}
		if len([]rune(content)) > 8192 {
			return "", "", apperror.InvalidArgument("content must be 8192 characters or fewer")
		}
		if !json.Valid([]byte(content)) {
			return "", "", apperror.InvalidArgument("content must be valid JSON for image/file messages")
		}
		if l.media == nil {
			return "", "", apperror.Internal("media validator is not configured")
		}
		if err := l.media.ValidateMessageMedia(ctx, senderID, contentType, content); err != nil {
			return "", "", err
		}
		return contentType, content, nil
	default:
		return "", "", apperror.InvalidArgument("content_type must be text, image, or file")
	}
}

func (l *MessageLogic) applyMessageOriginMetadata(input *repository.CreateMessageInput, req SendMessageRequest) error {
	origin := strings.ToLower(strings.TrimSpace(req.MessageOrigin))
	if origin == "" {
		origin = MessageOriginHuman
	}
	switch origin {
	case MessageOriginHuman, MessageOriginAI, MessageOriginSystem:
	default:
		return apperror.InvalidArgument("message_origin must be human, ai, or system")
	}
	input.MessageOrigin = origin
	input.AgentAccountID = strings.TrimSpace(req.AgentAccountID)
	input.TriggerServerMsgID = strings.TrimSpace(req.TriggerServerMsgID)
	input.AgentRunID = strings.TrimSpace(req.AgentRunID)
	input.AllowRecursiveTrigger = req.AllowRecursiveTrigger
	if origin != MessageOriginAI {
		if input.AgentAccountID != "" || input.TriggerServerMsgID != "" || input.AgentRunID != "" || input.AllowRecursiveTrigger {
			return apperror.InvalidArgument("agent metadata is only allowed for ai messages")
		}
		return nil
	}
	if input.AgentAccountID == "" {
		input.AgentAccountID = input.SenderID
	}
	if input.AgentAccountID != input.SenderID {
		return apperror.InvalidArgument("agent_account_id must match sender_id for ai messages")
	}
	return nil
}

func (l *MessageLogic) ensureUserExists(ctx context.Context, userID string) error {
	if l.userExists == nil {
		return nil
	}
	return l.userExists.EnsureUserExists(ctx, userID)
}

func (l *MessageLogic) resolveGroupParticipants(ctx context.Context, groupID string, senderID string) ([]string, error) {
	if err := l.ensureUserExists(ctx, senderID); err != nil {
		return nil, err
	}
	if l.groups == nil {
		return nil, apperror.Internal("group membership validator is not configured")
	}

	members, err := l.groups.ListMembers(ctx, ListMembersRequest{
		GroupID:         groupID,
		RequesterUserID: senderID,
	})
	if err != nil {
		return nil, err
	}

	participantIDs := make([]string, 0, len(members.Members))
	senderIsMember := false
	seen := make(map[string]struct{}, len(members.Members))
	for _, member := range members.Members {
		if member.State != "" && member.State != "active" {
			continue
		}
		userID := strings.TrimSpace(member.UserID)
		if userID == "" {
			continue
		}
		if _, ok := seen[userID]; ok {
			continue
		}
		seen[userID] = struct{}{}
		participantIDs = append(participantIDs, userID)
		if userID == senderID {
			senderIsMember = true
		}
	}
	if !senderIsMember {
		return nil, apperror.Forbidden("sender is not a group member")
	}
	return participantIDs, nil
}

func normalizeMessageRequiredID(value string, field string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", apperror.InvalidArgument(field + " is required")
	}
	if len([]rune(value)) > 128 {
		return "", apperror.InvalidArgument(field + " must be 128 characters or fewer")
	}
	if strings.Contains(value, "\x00") {
		return "", apperror.InvalidArgument(field + " cannot contain NUL")
	}
	return value, nil
}

func normalizeMessageConversationComponentID(value string, field string) (string, error) {
	value, err := normalizeMessageRequiredID(value, field)
	if err != nil {
		return "", err
	}
	if strings.Contains(value, ":") {
		return "", apperror.InvalidArgument(field + " cannot contain ':'")
	}
	return value, nil
}

func normalizeConversationID(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", apperror.InvalidArgument("conversation_id is required")
	}
	if len([]rune(value)) > 256 {
		return "", apperror.InvalidArgument("conversation_id must be 256 characters or fewer")
	}
	if strings.Contains(value, "\x00") {
		return "", apperror.InvalidArgument("conversation_id cannot contain NUL")
	}
	return value, nil
}

func normalizePullRange(fromSeq, toSeq int64, limit int, order string) (int64, int64, int, string, error) {
	if fromSeq < 0 {
		return 0, 0, 0, "", apperror.InvalidArgument("from_seq must be greater than or equal to 0")
	}
	if toSeq < 0 {
		return 0, 0, 0, "", apperror.InvalidArgument("to_seq must be greater than or equal to 0")
	}
	if fromSeq == 0 {
		fromSeq = 1
	}
	if limit < 0 {
		return 0, 0, 0, "", apperror.InvalidArgument("limit must be greater than or equal to 0")
	}
	if limit == 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}
	order = strings.ToLower(strings.TrimSpace(order))
	if order == "" {
		order = "asc"
	}
	if order != "asc" && order != "desc" {
		return 0, 0, 0, "", apperror.InvalidArgument("order must be asc or desc")
	}
	return fromSeq, toSeq, limit, order, nil
}

func messageCreatedHookEventID(message Message) string {
	if strings.TrimSpace(message.ServerMsgID) == "" {
		return ""
	}
	return "message.created:" + message.ServerMsgID
}
