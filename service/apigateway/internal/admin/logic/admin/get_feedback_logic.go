// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package admin

import (
	"context"

	"github.com/wujunhui99/agents_im/pkg/rpcerror"
	"github.com/wujunhui99/agents_im/service/apigateway/internal/admin/svc"
	"github.com/wujunhui99/agents_im/service/apigateway/internal/admin/types"
	adminpb "github.com/wujunhui99/agents_im_protocol/admin/v1"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetFeedbackLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetFeedbackLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetFeedbackLogic {
	return &GetFeedbackLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetFeedbackLogic) GetFeedback(req *types.AdminFeedbackReq) (resp *types.AdminFeedbackDetailResp, err error) {
	out, err := l.svcCtx.AdminRPC.GetFeedback(l.ctx, &adminpb.FeedbackDetailRequest{FeedbackId: req.FeedbackID})
	if err != nil {
		return nil, rpcerror.FromStatus(err)
	}
	return &types.AdminFeedbackDetailResp{Code: codeOK, Message: messageOK, Data: types.AdminFeedbackDetailData{Feedback: adminFeedback(out.GetFeedback())}}, nil
}
