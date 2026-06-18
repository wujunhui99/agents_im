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

type MarkConversationAsReadLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewMarkConversationAsReadLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MarkConversationAsReadLogic {
	return &MarkConversationAsReadLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *MarkConversationAsReadLogic) MarkConversationAsRead(req *types.MarkConversationAsReadReq) (resp *types.MarkConversationAsReadResp, err error) {
	userID, err := ctxuser.UserID(l.ctx)
	if err != nil {
		return nil, err
	}

	result, err := l.svcCtx.MsgRPC.MarkConversationAsRead(l.ctx, &msgpb.MarkConversationAsReadRequest{
		UserId:         userID,
		ConversationId: req.ConversationID,
		HasReadSeq:     req.HasReadSeq,
	})
	if err != nil {
		return nil, rpcerror.FromStatus(err)
	}
	return &types.MarkConversationAsReadResp{
		Code:    string(apperror.CodeOK),
		Message: "ok",
		Data: types.MarkConversationAsReadData{
			ConversationID: result.GetConversationId(),
			HasReadSeq:     result.GetHasReadSeq(),
			MaxSeq:         result.GetMaxSeq(),
			UnreadCount:    result.GetUnreadCount(),
			Updated:        result.GetUpdated(),
		},
	}, nil
}
