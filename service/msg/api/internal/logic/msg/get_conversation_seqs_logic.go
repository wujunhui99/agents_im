// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package msg

import (
	"context"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/pkg/ctxuser"
	"github.com/wujunhui99/agents_im/pkg/rpcerror"
	"github.com/wujunhui99/agents_im/service/msg/api/internal/svc"
	"github.com/wujunhui99/agents_im/service/msg/api/internal/types"
	msgpb "github.com/wujunhui99/agents_im/service/msg/rpc/msg"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetConversationSeqsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetConversationSeqsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetConversationSeqsLogic {
	return &GetConversationSeqsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetConversationSeqsLogic) GetConversationSeqs(req *types.ConversationSeqsReq) (resp *types.ConversationSeqsResp, err error) {
	userID, err := ctxuser.UserID(l.ctx)
	if err != nil {
		return nil, err
	}

	result, err := l.svcCtx.MsgRPC.GetConversationsSeqState(l.ctx, &msgpb.GetConversationsSeqStateRequest{
		UserId:          userID,
		ConversationIds: splitCommaQuery(req.ConversationIDs),
	})
	if err != nil {
		return nil, rpcerror.FromStatus(err)
	}

	states := make([]types.ConversationSeqState, 0, len(result.GetStates()))
	for _, s := range result.GetStates() {
		states = append(states, pbToSeqState(s))
	}
	return &types.ConversationSeqsResp{
		Code:    string(apperror.CodeOK),
		Message: "ok",
		Data:    types.ConversationSeqsData{States: states},
	}, nil
}
