package message

import (
	"context"

	"github.com/wujunhui99/agents_im/internal/apperror"
	"github.com/wujunhui99/agents_im/internal/ctxuser"
	business "github.com/wujunhui99/agents_im/internal/logic"
	messagesvc "github.com/wujunhui99/agents_im/internal/servicecontext/message"
	"github.com/wujunhui99/agents_im/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetConversationSeqsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *messagesvc.ServiceContext
}

func NewGetConversationSeqsLogic(ctx context.Context, svcCtx *messagesvc.ServiceContext) *GetConversationSeqsLogic {
	return &GetConversationSeqsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetConversationSeqsLogic) GetConversationSeqs(req *types.ConversationSeqsReq) (*types.ConversationSeqsResp, error) {
	userID, err := ctxuser.UserID(l.ctx)
	if err != nil {
		return nil, err
	}

	result, err := l.svcCtx.MessageLogic.GetConversationSeqs(l.ctx, business.GetConversationSeqsRequest{
		UserID:          userID,
		ConversationIDs: splitCommaQuery(req.ConversationIDs),
	})
	if err != nil {
		return nil, err
	}

	states := make([]types.ConversationSeqState, 0, len(result.States))
	for _, state := range result.States {
		states = append(states, toConversationSeqState(state))
	}
	return &types.ConversationSeqsResp{
		Code:    string(apperror.CodeOK),
		Message: "ok",
		Data:    types.ConversationSeqsData{States: states},
	}, nil
}
