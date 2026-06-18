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

type ListFeedbackLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListFeedbackLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListFeedbackLogic {
	return &ListFeedbackLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListFeedbackLogic) ListFeedback(req *types.AdminFeedbackListReq) (resp *types.AdminFeedbackListResp, err error) {
	out, err := l.svcCtx.AdminRPC.ListFeedback(l.ctx, &adminpb.FeedbackListRequest{
		Status: req.Status,
		Limit:  int32(req.Limit),
		Offset: int32(req.Offset),
	})
	if err != nil {
		return nil, rpcerror.FromStatus(err)
	}
	return &types.AdminFeedbackListResp{Code: codeOK, Message: messageOK, Data: types.AdminFeedbackListData{Items: adminFeedbackItems(out.GetItems())}}, nil
}
