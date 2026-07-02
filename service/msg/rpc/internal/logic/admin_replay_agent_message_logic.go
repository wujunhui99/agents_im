package logic

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/pkg/messaging"
	"github.com/wujunhui99/agents_im/pkg/observability"
	"github.com/wujunhui99/agents_im/pkg/rpcerror"
	"github.com/wujunhui99/agents_im/service/msg/rpc/internal/model"
	"github.com/wujunhui99/agents_im/service/msg/rpc/internal/svc"
	"github.com/wujunhui99/agents_im/service/msg/rpc/msg"

	"github.com/zeromicro/go-zero/core/logx"
)

type AdminReplayAgentMessageLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAdminReplayAgentMessageLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminReplayAgentMessageLogic {
	return &AdminReplayAgentMessageLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// AdminReplayAgentMessage 重放一条人类消息以重新触发 agent（#618，属主侧）。
//
// 旧实现（internal/logic.MessageCreatedHook）是 message monolith keystone，拆分部署后
// 该 hook 未接线（休眠，返回错误）。这里由消息属主 msg-rpc 直接重新 publish 一条
// message.accepted 到 agent.trigger.v1——与 msgtransfer 正常触发链路同源。幂等收敛在
// agent 侧 agent_triggers 台账（RequestID = event_id:agent），EventID 取 server_msg_id
// 稳定派生，故重复重放不会二次回复。
func (l *AdminReplayAgentMessageLogic) AdminReplayAgentMessage(in *msg.AdminReplayAgentMessageRequest) (*msg.AdminReplayAgentMessageResponse, error) {
	conversationID, err := normalizeConversationID(in.GetConversationId())
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	serverMsgID := strings.TrimSpace(in.GetServerMsgId())
	if serverMsgID == "" {
		return nil, rpcerror.ToStatus(apperror.InvalidArgument("server_msg_id is required"))
	}
	messageID, err := strconv.ParseInt(serverMsgID, 10, 64)
	if err != nil {
		return nil, rpcerror.ToStatus(apperror.InvalidArgument("server_msg_id must be a numeric message id"))
	}

	row, err := l.svcCtx.Messages.FindOne(l.ctx, messageID)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, rpcerror.ToStatus(apperror.NotFound("message not found in conversation"))
		}
		return nil, rpcerror.ToStatus(err)
	}
	if row.ConversationId != conversationID {
		return nil, rpcerror.ToStatus(apperror.NotFound("message not found in conversation"))
	}

	// 触发前置校验（与旧 admin replay 对齐）：只有发往某 agent 账号的人类文本单聊可重放。
	if model.MessageOriginString(row.MessageOrigin) != model.MessageOriginHuman {
		return nil, rpcerror.ToStatus(apperror.InvalidArgument("only human messages can be replayed for AI triggering"))
	}
	chatType := model.ConversationTypeString(row.ConversationType)
	if chatType != model.ChatTypeSingle || strings.TrimSpace(row.ReceiverAccountId) == "" {
		return nil, rpcerror.ToStatus(apperror.InvalidArgument("only direct messages to an agent account can be replayed"))
	}
	if model.ContentTypeString(row.ContentType) != model.ContentTypeText {
		return nil, rpcerror.ToStatus(apperror.InvalidArgument("only text messages can be replayed for AI triggering"))
	}

	trace := observability.TraceContextFromContext(l.ctx)
	sendTime := model.MessageSendTime(row)
	event := messaging.MessageEvent{
		EventID:        "admin.replay.message.accepted:" + serverMsgID,
		EventType:      messaging.EventTypeMessageAccepted,
		ConversationID: row.ConversationId,
		ServerMsgID:    serverMsgID,
		Seq:            row.Seq,
		SenderID:       row.SenderAccountId,
		ChatType:       chatType,
		CreatedAt:      sendTime,
		Payload: messaging.MessageEventPayload{
			ClientMsgID:           row.ClientMsgId,
			ReceiverID:            row.ReceiverAccountId,
			ReceiverIDs:           model.VisibleUserIDs(row.SenderAccountId, row.ReceiverAccountId, chatType, nil),
			ContentType:           model.ContentTypeString(row.ContentType),
			Content:               json.RawMessage(row.Content),
			MessageOrigin:         model.MessageOriginString(row.MessageOrigin),
			AgentAccountID:        row.AgentAccountId,
			TriggerServerMsgID:    row.TriggerMessageId,
			AgentRunID:            row.AgentRunId,
			AllowRecursiveTrigger: row.AllowRecursiveTrigger,
			VisibleUserIDs:        model.VisibleUserIDs(row.SenderAccountId, row.ReceiverAccountId, chatType, nil),
			PayloadHash:           row.PayloadHash,
			SendTime:              sendTime,
			TraceID:               trace.TraceID,
			RequestID:             trace.RequestID,
			TraceParent:           trace.TraceParent,
			TraceState:            trace.TraceState,
		},
	}
	if err := l.svcCtx.Producer.PublishEvent(l.ctx, messaging.TopicAgentTrigger, event); err != nil {
		l.Errorf("admin replay publish failed conversation_id=%q server_msg_id=%q: %v", conversationID, serverMsgID, err)
		return nil, rpcerror.ToStatus(apperror.Internal("message queue unavailable"))
	}

	return &msg.AdminReplayAgentMessageResponse{
		ConversationId: conversationID,
		ServerMsgId:    serverMsgID,
		Triggered:      true,
		Message:        messageToPB(row),
	}, nil
}
