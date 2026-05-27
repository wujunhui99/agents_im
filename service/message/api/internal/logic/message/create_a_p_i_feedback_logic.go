// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package message

import (
	"context"

	"github.com/wujunhui99/agents_im/service/message/api/internal/svc"
	"github.com/wujunhui99/agents_im/service/message/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateAPIFeedbackLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateAPIFeedbackLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateAPIFeedbackLogic {
	return &CreateAPIFeedbackLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreateAPIFeedbackLogic) CreateAPIFeedback(req *types.CreateFeedbackReq) (resp *types.CreateFeedbackResp, err error) {
	return NewCreateFeedbackLogic(l.ctx, l.svcCtx).CreateFeedback(req)
}
