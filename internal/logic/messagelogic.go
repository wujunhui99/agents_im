package logic

import (
	"context"
	"strings"

	"github.com/wujunhui99/agents_im/internal/apperror"
	"github.com/wujunhui99/agents_im/internal/observability"
	"github.com/wujunhui99/agents_im/internal/repository"
)

const (
	MessageChatTypeSingle = repository.ChatTypeSingle
	MessageChatTypeGroup  = repository.ChatTypeGroup

	MessageContentTypeText = repository.ContentTypeText
)

type GroupMemberLister interface {
	ListMembers(ctx context.Context, req ListMembersRequest) (ListMembersResponse, error)
}

type MessageLogic struct {
	repo       repository.MessageRepository
	userExists UserExistenceChecker
	groups     GroupMemberLister
}

func NewMessageLogic(repo repository.MessageRepository) *MessageLogic {
	return &MessageLogic{repo: repo}
}

func NewMessageLogicWithValidators(repo repository.MessageRepository, userExists UserExistenceChecker, groups GroupMemberLister) *MessageLogic {
	return &MessageLogic{repo: repo, userExists: userExists, groups: groups}
}

type Message = repository.Message

type ConversationSeqState = repository.ConversationSeqState

type SendMessageRequest struct {
	SenderID    string `json:"senderId"`
	ReceiverID  string `json:"receiverId"`
	GroupID     string `json:"groupId"`
	ChatType    string `json:"chatType"`
	ClientMsgID string `json:"clientMsgId"`
	ContentType string `json:"contentType"`
	Content     string `json:"content"`
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

	return SendMessageResponse{
		Message:          message,
		Deduplicated:     deduplicated,
		RecipientUserIDs: repository.DeliveryRecipientUserIDs(input),
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
	senderID, err := normalizeMessageRequiredID(req.SenderID, "sender_id")
	if err != nil {
		return repository.CreateMessageInput{}, err
	}
	chatType := strings.ToLower(strings.TrimSpace(req.ChatType))
	clientMsgID, err := normalizeMessageRequiredID(req.ClientMsgID, "client_msg_id")
	if err != nil {
		return repository.CreateMessageInput{}, err
	}
	contentType := strings.ToLower(strings.TrimSpace(req.ContentType))
	if contentType != MessageContentTypeText {
		return repository.CreateMessageInput{}, apperror.InvalidArgument("content_type must be text")
	}
	content := strings.TrimSpace(req.Content)
	if content == "" {
		return repository.CreateMessageInput{}, apperror.InvalidArgument("content is required")
	}
	if len([]rune(content)) > 4096 {
		return repository.CreateMessageInput{}, apperror.InvalidArgument("content must be 4096 characters or fewer")
	}

	input := repository.CreateMessageInput{
		SenderID:    senderID,
		ChatType:    chatType,
		ClientMsgID: clientMsgID,
		ContentType: contentType,
		Content:     content,
	}

	switch chatType {
	case MessageChatTypeSingle:
		receiverID, err := normalizeMessageRequiredID(req.ReceiverID, "receiver_id")
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
		groupID, err := normalizeMessageRequiredID(req.GroupID, "group_id")
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
