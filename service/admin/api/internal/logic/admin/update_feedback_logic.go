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

type UpdateFeedbackLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateFeedbackLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateFeedbackLogic {
	return &UpdateFeedbackLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateFeedbackLogic) UpdateFeedback(req *types.AdminFeedbackUpdateReq) (resp *types.AdminFeedbackDetailResp, err error) {
	out, err := l.svcCtx.AdminRPC.UpdateFeedback(l.ctx, &adminpb.FeedbackUpdateRequest{
		FeedbackId: req.FeedbackID,
		Status:     req.Status,
		AdminNote:  req.AdminNote,
	})
	if err != nil {
		return nil, rpcerror.FromStatus(err)
	}
	return &types.AdminFeedbackDetailResp{Code: codeOK, Message: messageOK, Data: types.AdminFeedbackDetailData{Feedback: adminFeedback(out.GetFeedback())}}, nil
}
