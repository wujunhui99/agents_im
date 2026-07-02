package logic

import (
	"context"
	"strings"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/pkg/rpcerror"
	"github.com/wujunhui99/agents_im/service/msg/rpc/internal/groupsrpc"
	"github.com/wujunhui99/agents_im/service/msg/rpc/internal/model"
	"github.com/wujunhui99/agents_im/service/msg/rpc/internal/svc"
	"github.com/wujunhui99/agents_im/service/msg/rpc/msg"

	"github.com/zeromicro/go-zero/core/logx"
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

// SendMessage 唯一写原语 = publish Kafka（03 §9 B2/B3b）：旧「PG 事务内分配 seq +
// 写消息/会话/已读/outbox」同步路径已退役——seq 分配与落库归 msgtransfer，
// dedup 收敛在 msgtransfer，AI 触发经 agent.trigger.v1 回流。
func (l *SendMessageLogic) SendMessage(in *msg.SendMessageRequest) (*msg.SendMessageResponse, error) {
	ns, err := l.normalize(in)
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	if l.svcCtx.Producer == nil {
		// 启动期 svc.NewServiceContext 已 Fatalf 兜底；这里防手工构造的 svcCtx 误用。
		return nil, rpcerror.ToStatus(apperror.Internal("kafka producer is not configured"))
	}
	return l.sendDirectKafka(ns, messagePayloadHash(ns))
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

// resolveGroupParticipants 解析群成员并校验发送者在群内（经 groups-rpc ListMembers，#617）。
func (l *SendMessageLogic) resolveGroupParticipants(groupID, senderID string) ([]string, error) {
	if l.svcCtx.Groups == nil {
		return nil, apperror.Internal("group membership validator is not configured")
	}
	members, err := l.svcCtx.Groups.ListMembers(l.ctx, groupsrpc.ListMembersRequest{
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
