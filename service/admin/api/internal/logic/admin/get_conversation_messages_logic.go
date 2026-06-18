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

type GetConversationMessagesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetConversationMessagesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetConversationMessagesLogic {
	return &GetConversationMessagesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetConversationMessagesLogic) GetConversationMessages(req *types.AdminConversationMessagesReq) (resp *types.AdminConversationMessagesResp, err error) {
	out, err := l.svcCtx.AdminRPC.GetConversationMessages(l.ctx, &adminpb.ConversationMessagesRequest{
		ConversationId: req.ConversationID,
		FromSeq:        req.FromSeq,
		ToSeq:          req.ToSeq,
		Limit:          int32(req.Limit),
		Order:          req.Order,
	})
	if err != nil {
		return nil, rpcerror.FromStatus(err)
	}
	return &types.AdminConversationMessagesResp{Code: codeOK, Message: messageOK, Data: types.AdminConversationMessagesData{
		ConversationID: out.GetConversationId(),
		Messages:       adminMessages(out.GetMessages()),
		IsEnd:          out.GetIsEnd(),
		NextSeq:        out.GetNextSeq(),
	}}, nil
}
