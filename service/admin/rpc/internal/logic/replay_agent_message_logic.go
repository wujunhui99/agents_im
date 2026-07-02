package logic

import (
	"context"
	"strings"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/pkg/rpcerror"
	"github.com/wujunhui99/agents_im/service/admin/rpc/admin"
	"github.com/wujunhui99/agents_im/service/admin/rpc/internal/svc"
	msgpb "github.com/wujunhui99/agents_im/service/msg/rpc/msg"

	"github.com/zeromicro/go-zero/core/logx"
)

type ReplayAgentMessageLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewReplayAgentMessageLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ReplayAgentMessageLogic {
	return &ReplayAgentMessageLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

// ReplayAgentMessage 重放一条人类消息以触发 agent 响应。
// 触发由消息属主 msg-rpc.AdminReplayAgentMessage 执行（#618，重发 agent.trigger.v1，脱旧
// MessageCreatedHook keystone）；校验/幂等收敛在属主与 agent_triggers 台账。
func (l *ReplayAgentMessageLogic) ReplayAgentMessage(in *admin.ReplayAgentMessageRequest) (*admin.ReplayAgentMessageResponse, error) {
	conversationID, err := validateRequiredAdminID(in.GetConversationId(), "conversation_id", 256)
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	serverMsgID := strings.TrimSpace(in.GetServerMsgId())
	if serverMsgID == "" {
		return nil, rpcerror.ToStatus(apperror.InvalidArgument("server_msg_id is required"))
	}

	resp, err := l.svcCtx.MsgRPC.AdminReplayAgentMessage(l.ctx, &msgpb.AdminReplayAgentMessageRequest{
		ConversationId: conversationID,
		ServerMsgId:    serverMsgID,
	})
	if err != nil {
		return nil, rpcerror.ToStatus(rpcerror.FromStatus(err))
	}
	return &admin.ReplayAgentMessageResponse{
		ConversationId: resp.GetConversationId(),
		ServerMsgId:    resp.GetServerMsgId(),
		Triggered:      resp.GetTriggered(),
		Skipped:        resp.GetSkipped(),
		Reason:         resp.GetReason(),
		Message:        adminMessagePB(resp.GetMessage()),
	}, nil
}
