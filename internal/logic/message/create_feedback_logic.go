package message

import (
	"context"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/pkg/ctxuser"
	business "github.com/wujunhui99/agents_im/internal/logic"
	messagesvc "github.com/wujunhui99/agents_im/internal/servicecontext/message"
	"github.com/wujunhui99/agents_im/common/share/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type CreateFeedbackLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *messagesvc.ServiceContext
}

func NewCreateFeedbackLogic(ctx context.Context, svcCtx *messagesvc.ServiceContext) *CreateFeedbackLogic {
	return &CreateFeedbackLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *CreateFeedbackLogic) CreateFeedback(req *types.CreateFeedbackReq) (*types.CreateFeedbackResp, error) {
	if l == nil || l.svcCtx == nil || l.svcCtx.FeedbackLogic == nil {
		return nil, apperror.Internal("feedback logic is not configured")
	}
	userID, err := ctxuser.UserID(l.ctx)
	if err != nil {
		return nil, err
	}
	created, err := l.svcCtx.FeedbackLogic.CreateFeedback(l.ctx, business.CreateFeedbackRequest{
		UserID:     userID,
		Category:   req.Category,
		Title:      req.Title,
		Content:    req.Content,
		Contact:    req.Contact,
		PageURL:    req.PageURL,
		UserAgent:  req.UserAgent,
		ClientMeta: req.ClientMeta,
	})
	if err != nil {
		return nil, err
	}
	return &types.CreateFeedbackResp{
		Code:    string(apperror.CodeOK),
		Message: "ok",
		Data: types.FeedbackData{
			FeedbackID: created.FeedbackID,
			Status:     string(created.Status),
		},
	}, nil
}
