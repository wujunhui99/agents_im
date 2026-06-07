package logic

import (
	"context"
	"strings"

	"github.com/wujunhui99/agents_im/common/share/rpcerror"
	msglogic "github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/repository"
	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/service/admin/rpc/admin"
	"github.com/wujunhui99/agents_im/service/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

const adminAIReplayMessageLimit = 500

type ReplayAgentMessageLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewReplayAgentMessageLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ReplayAgentMessageLogic {
	return &ReplayAgentMessageLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

// ReplayAgentMessage 重放一条人类消息以触发 agent 响应。
// 触发依赖 MessageCreatedHook（message monolith keystone）；独立 admin 二进制中该 hook 未接线（休眠）。
func (l *ReplayAgentMessageLogic) ReplayAgentMessage(in *admin.ReplayAgentMessageRequest) (*admin.ReplayAgentMessageResponse, error) {
	if l.svcCtx.Messages == nil {
		return nil, rpcerror.ToStatus(apperror.Internal("admin AI replay message repository is not configured"))
	}
	conversationID, err := validateRequiredAdminID(in.GetConversationId(), "conversation_id", 256)
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	serverMsgID := strings.TrimSpace(in.GetServerMsgId())
	if serverMsgID == "" {
		return nil, rpcerror.ToStatus(apperror.InvalidArgument("server_msg_id is required"))
	}

	messages, _, _, err := l.svcCtx.Messages.GetMessages(l.ctx, conversationID, 1, 0, adminAIReplayMessageLimit, repository.MessageStorageOrderAsc)
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	var trigger repository.Message
	found := false
	for _, message := range messages {
		if message.ServerMsgID == serverMsgID {
			trigger = message
			found = true
		}
		if message.TriggerServerMsgID == serverMsgID && message.MessageOrigin == msglogic.MessageOriginAI {
			return &admin.ReplayAgentMessageResponse{
				ConversationId: conversationID,
				ServerMsgId:    serverMsgID,
				Skipped:        true,
				Reason:         "ai response already exists for trigger message",
				Message:        adminMessagePB(message),
			}, nil
		}
	}
	if !found {
		return nil, rpcerror.ToStatus(apperror.NotFound("message not found in conversation"))
	}
	if trigger.MessageOrigin != msglogic.MessageOriginHuman {
		return nil, rpcerror.ToStatus(apperror.InvalidArgument("only human messages can be replayed for AI triggering"))
	}
	if trigger.ChatType != msglogic.MessageChatTypeSingle || strings.TrimSpace(trigger.ReceiverID) == "" {
		return nil, rpcerror.ToStatus(apperror.InvalidArgument("only direct messages to an agent account can be replayed"))
	}
	if trigger.ContentType != msglogic.MessageContentTypeText {
		return nil, rpcerror.ToStatus(apperror.InvalidArgument("only text messages can be replayed for AI triggering"))
	}
	if l.svcCtx.MessageCreatedHook == nil {
		return nil, rpcerror.ToStatus(apperror.Internal("message created hook is not configured"))
	}

	eventID := "admin.replay.message.created:" + trigger.ServerMsgID
	if err := l.svcCtx.MessageCreatedHook.OnMessageCreated(l.ctx, msglogic.MessageCreatedHookInput{
		EventID:               eventID,
		Message:               trigger.Clone(),
		TargetAgentAccountIDs: []string{trigger.ReceiverID},
	}); err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	return &admin.ReplayAgentMessageResponse{
		ConversationId: conversationID,
		ServerMsgId:    serverMsgID,
		Triggered:      true,
		Message:        adminMessagePB(trigger),
	}, nil
}
