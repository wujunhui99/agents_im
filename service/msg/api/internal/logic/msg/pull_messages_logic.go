// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package msg

import (
	"context"

	"github.com/wujunhui99/agents_im/common/share/rpcerror"
	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/pkg/ctxuser"
	"github.com/wujunhui99/agents_im/service/msg/api/internal/svc"
	"github.com/wujunhui99/agents_im/service/msg/api/internal/types"
	msgpb "github.com/wujunhui99/agents_im/service/msg/rpc/msg"

	"github.com/zeromicro/go-zero/core/logx"
)

type PullMessagesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewPullMessagesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PullMessagesLogic {
	return &PullMessagesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PullMessagesLogic) PullMessages(req *types.PullMessagesReq) (resp *types.PullMessagesResp, err error) {
	userID, err := ctxuser.UserID(l.ctx)
	if err != nil {
		return nil, err
	}

	result, err := l.svcCtx.MsgRPC.PullMessages(l.ctx, &msgpb.PullMessagesRequest{
		UserId:         userID,
		ConversationId: req.ConversationID,
		FromSeq:        req.FromSeq,
		ToSeq:          req.ToSeq,
		Limit:          int32(req.Limit),
		Order:          req.Order,
	})
	if err != nil {
		return nil, rpcerror.FromStatus(err)
	}

	messages := make([]types.Message, 0, len(result.GetMessages()))
	for _, m := range result.GetMessages() {
		messages = append(messages, pbToMessage(m))
	}
	return &types.PullMessagesResp{
		Code:    string(apperror.CodeOK),
		Message: "ok",
		Data: types.PullMessagesData{
			Messages: messages,
			IsEnd:    result.GetIsEnd(),
			NextSeq:  result.GetNextSeq(),
		},
	}, nil
}
