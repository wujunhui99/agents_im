package logic

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/wujunhui99/agents_im/common/share/rpcerror"
	business "github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/repository"
	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/pkg/idgen"
	"github.com/wujunhui99/agents_im/service/msg/rpc/internal/model"
	"github.com/wujunhui99/agents_im/service/msg/rpc/internal/svc"
	"github.com/wujunhui99/agents_im/service/msg/rpc/msg"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type SendMessageLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSendMessageLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SendMessageLogic {
	return &SendMessageLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// SendMessage 行为对齐旧 message-rpc：PG 事务内分配 seq、写消息/会话/已读状态/outbox，幂等去重。
// Phase 0 不动 Kafka（07-message-rpc-redesign §4.2 的 MsgToMQ 改造留待 Phase 1）。
func (l *SendMessageLogic) SendMessage(in *msg.SendMessageRequest) (*msg.SendMessageResponse, error) {
	ns, err := l.normalize(in)
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	payloadHash := messagePayloadHash(ns)

	stored, deduplicated, err := l.persist(ns, payloadHash)
	if err != nil {
		if model.IsPostgresUniqueViolation(err) {
			existing, derr := l.svcCtx.Messages.FindBySenderClient(l.ctx, ns.SenderID, ns.ClientMsgID)
			if derr != nil {
				return nil, rpcerror.ToStatus(derr)
			}
			if existing.PayloadHash != payloadHash {
				return nil, rpcerror.ToStatus(apperror.AlreadyExists("idempotency conflict"))
			}
			l.fireMessageCreatedHook(ns, existing, true)
			return &msg.SendMessageResponse{Message: messageToPB(existing), Deduplicated: true}, nil
		}
		if model.IsPostgresCheckViolation(err) {
			return nil, rpcerror.ToStatus(apperror.InvalidArgument("invalid message"))
		}
		return nil, rpcerror.ToStatus(err)
	}

	l.fireMessageCreatedHook(ns, stored, deduplicated)
	return &msg.SendMessageResponse{Message: messageToPB(stored), Deduplicated: deduplicated}, nil
}

// fireMessageCreatedHook 在消息落库后触发 AI 托管钩子（keystone 例外：语义对齐原
// internal/logic.MessageLogic 的 messageCreatedHook 调用点——成功路径含 dedup 均触发、
// 钩子错误只记日志不影响 ACK；待 03-message-pipeline §9 B1 把触发点迁到 msgtransfer 后删除）。
func (l *SendMessageLogic) fireMessageCreatedHook(ns normalizedSend, stored *model.Messages, deduplicated bool) {
	if l.svcCtx.AgentHook == nil || stored == nil {
		return
	}
	message := messageToBusiness(stored)
	eventID := ""
	if strings.TrimSpace(message.ServerMsgID) != "" {
		eventID = "message.created:" + message.ServerMsgID
	}
	recipients := repository.DeliveryRecipientUserIDs(repository.CreateMessageInput{
		SenderID:           ns.SenderID,
		ReceiverID:         ns.ReceiverID,
		GroupID:            ns.GroupID,
		ChatType:           ns.ChatType,
		MessageOrigin:      ns.MessageOrigin,
		ParticipantUserIDs: ns.ParticipantUserIDs,
	})
	if err := l.svcCtx.AgentHook.OnMessageCreated(l.ctx, business.MessageCreatedHookInput{
		EventID:          eventID,
		Message:          message.Clone(),
		Deduplicated:     deduplicated,
		RecipientUserIDs: recipients,
	}); err != nil {
		l.Errorf("message created hook failed after message accepted server_msg_id=%q conversation_id=%q seq=%d: %v",
			message.ServerMsgID, message.ConversationID, message.Seq, err)
	}
}

// persist 在单事务内完成幂等写入（移植自 internal/repository.CreateMessageIdempotent）。
func (l *SendMessageLogic) persist(ns normalizedSend, payloadHash string) (*model.Messages, bool, error) {
	var stored *model.Messages
	deduplicated := false
	err := l.svcCtx.Messages.Transact(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		msgs := l.svcCtx.Messages.WithSession(session)
		threads := l.svcCtx.Threads.WithSession(session)
		states := l.svcCtx.States.WithSession(session)
		outbox := l.svcCtx.Outbox.WithSession(session)

		existing, err := msgs.FindBySenderClient(ctx, ns.SenderID, ns.ClientMsgID)
		if err == nil {
			if existing.PayloadHash != payloadHash {
				return apperror.AlreadyExists("idempotency conflict")
			}
			stored = existing
			deduplicated = true
			return nil
		}
		if !errors.Is(err, model.ErrNotFound) {
			return err
		}

		maxSeq, err := threads.UpsertAndLock(ctx, ns.ConversationID, model.UpsertConversationParams{
			ChatType:    ns.ChatType,
			SingleUserA: ns.SenderID,
			SingleUserB: ns.ReceiverID,
			GroupID:     ns.GroupID,
		})
		if err != nil {
			return err
		}
		nextSeq := maxSeq + 1
		sendTime := time.Now().UTC()

		messageID, err := idgen.NewString()
		if err != nil {
			return err
		}
		contentJSON, err := model.EncodeMessageContent(ns.ContentType, ns.Content)
		if err != nil {
			return err
		}
		conversationType, err := model.ConversationTypeValue(ns.ChatType)
		if err != nil {
			return err
		}
		contentType, err := model.ContentTypeValue(ns.ContentType)
		if err != nil {
			return err
		}
		messageOrigin, err := model.MessageOriginValue(ns.MessageOrigin)
		if err != nil {
			return err
		}

		inserted, err := msgs.InsertReturning(ctx, &model.Messages{
			MessageId:             messageID,
			ClientMsgId:           ns.ClientMsgID,
			SenderAccountId:       ns.SenderID,
			ConversationId:        ns.ConversationID,
			Seq:                   nextSeq,
			ConversationType:      conversationType,
			ReceiverAccountId:     ns.ReceiverID,
			GroupId:               ns.GroupID,
			ContentType:           contentType,
			Content:               contentJSON,
			MessageOrigin:         messageOrigin,
			AgentAccountId:        ns.AgentAccountID,
			TriggerMessageId:      ns.TriggerServerMsgID,
			AgentRunId:            ns.AgentRunID,
			AllowRecursiveTrigger: ns.AllowRecursiveTrigger,
			PayloadHash:           payloadHash,
			ClientSendTime:        sql.NullTime{Time: sendTime, Valid: true},
		})
		if err != nil {
			return err
		}

		visibleStartSeq := int64(0)
		if ns.ChatType == model.ChatTypeGroup {
			visibleStartSeq = maxSeq
		}
		visible := model.VisibleUserIDs(ns.SenderID, ns.ReceiverID, ns.ChatType, ns.ParticipantUserIDs)
		if err := states.UpsertVisible(ctx, visible, ns.ConversationID, visibleStartSeq); err != nil {
			return err
		}
		if err := states.UpsertSenderRead(ctx, ns.SenderID, ns.ConversationID, nextSeq); err != nil {
			return err
		}
		if err := threads.UpdateAfterMessage(ctx, ns.ConversationID, messageID, nextSeq, sendTime); err != nil {
			return err
		}

		payload, err := buildMessageCreatedOutboxPayload(ctx, inserted, visible)
		if err != nil {
			return err
		}
		eventID, err := idgen.NewString()
		if err != nil {
			return err
		}
		if err := outbox.InsertCreatedEvent(ctx, model.OutboxCreatedEventParams{
			EventID:        eventID,
			ConversationID: ns.ConversationID,
			MessageID:      messageID,
			Seq:            nextSeq,
			Payload:        payload,
		}); err != nil {
			return err
		}

		stored = inserted
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	return stored, deduplicated, nil
}

func (l *SendMessageLogic) normalize(in *msg.SendMessageRequest) (normalizedSend, error) {
	senderID, err := normalizeMessageConversationComponentID(in.GetSenderId(), "sender_id")
	if err != nil {
		return normalizedSend{}, err
	}
	chatType := strings.ToLower(strings.TrimSpace(in.GetChatType()))
	clientMsgID, err := normalizeMessageRequiredID(in.GetClientMsgId(), "client_msg_id")
	if err != nil {
		return normalizedSend{}, err
	}
	contentType, content, err := normalizeMessageContent(l.ctx, l.svcCtx.Media, senderID, in.GetContentType(), in.GetContent())
	if err != nil {
		return normalizedSend{}, err
	}

	ns := normalizedSend{
		SenderID:    senderID,
		ChatType:    chatType,
		ClientMsgID: clientMsgID,
		ContentType: contentType,
		Content:     content,
	}
	if err := applyMessageOriginMetadata(&ns, in.GetMessageOrigin(), in.GetAgentAccountId(), in.GetTriggerServerMsgId(), in.GetAgentRunId(), in.GetAllowRecursiveTrigger()); err != nil {
		return normalizedSend{}, err
	}

	switch chatType {
	case model.ChatTypeSingle:
		receiverID, err := normalizeMessageConversationComponentID(in.GetReceiverId(), "receiver_id")
		if err != nil {
			return normalizedSend{}, err
		}
		if senderID == receiverID {
			return normalizedSend{}, apperror.InvalidArgument("sender_id and receiver_id must be different")
		}
		if strings.TrimSpace(in.GetGroupId()) != "" {
			return normalizedSend{}, apperror.InvalidArgument("group_id must be empty for single chat")
		}
		ns.ReceiverID = receiverID
		ns.ParticipantUserIDs = []string{senderID, receiverID}
		ns.ConversationID = model.SingleConversationID(senderID, receiverID)
	case model.ChatTypeGroup:
		groupID, err := normalizeMessageConversationComponentID(in.GetGroupId(), "group_id")
		if err != nil {
			return normalizedSend{}, err
		}
		if strings.TrimSpace(in.GetReceiverId()) != "" {
			return normalizedSend{}, apperror.InvalidArgument("receiver_id must be empty for group chat")
		}
		participants, err := l.resolveGroupParticipants(groupID, senderID)
		if err != nil {
			return normalizedSend{}, err
		}
		ns.GroupID = groupID
		ns.ParticipantUserIDs = participants
		ns.ConversationID = model.GroupConversationID(groupID)
	default:
		return normalizedSend{}, apperror.InvalidArgument("chat_type must be single or group")
	}

	if _, err := normalizeConversationID(ns.ConversationID); err != nil {
		return normalizedSend{}, err
	}
	return ns, nil
}

// resolveGroupParticipants 解析群成员并校验发送者在群内（keystone 例外：调 internal GroupsLogic）。
func (l *SendMessageLogic) resolveGroupParticipants(groupID, senderID string) ([]string, error) {
	if l.svcCtx.Groups == nil {
		return nil, apperror.Internal("group membership validator is not configured")
	}
	members, err := l.svcCtx.Groups.ListMembers(l.ctx, business.ListMembersRequest{
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
