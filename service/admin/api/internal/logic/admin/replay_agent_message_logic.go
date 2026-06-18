// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package admin

import (
	"context"

	"github.com/wujunhui99/agents_im/pkg/rpcerror"
	"github.com/wujunhui99/agents_im/service/admin/api/internal/svc"
	"github.com/wujunhui99/agents_im/service/admin/api/internal/types"
	adminpb "github.com/wujunhui99/agents_im/service/admin/rpc/admin"

	"github.com/zeromicro/go-zero/core/logx"
)

type ReplayAgentMessageLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewReplayAgentMessageLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ReplayAgentMessageLogic {
	return &ReplayAgentMessageLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ReplayAgentMessageLogic) ReplayAgentMessage(req *types.AdminReplayAgentMessageReq) (resp *types.AdminReplayAgentMessageResp, err error) {
	out, err := l.svcCtx.AdminRPC.ReplayAgentMessage(l.ctx, &adminpb.ReplayAgentMessageRequest{
		ConversationId: req.ConversationID,
		ServerMsgId:    req.ServerMsgID,
	})
	if err != nil {
		return nil, rpcerror.FromStatus(err)
	}
	return &types.AdminReplayAgentMessageResp{Code: codeOK, Message: messageOK, Data: types.AdminReplayAgentMessageData{
		ConversationID: out.GetConversationId(),
		ServerMsgID:    out.GetServerMsgId(),
		Triggered:      out.GetTriggered(),
		Skipped:        out.GetSkipped(),
		Reason:         out.GetReason(),
		Message:        adminMessage(out.GetMessage()),
	}}, nil
}
