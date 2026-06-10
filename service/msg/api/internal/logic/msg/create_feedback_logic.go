// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package msg

import (
	"context"

	"github.com/wujunhui99/agents_im/common/share/rpcerror"
	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/pkg/ctxuser"
	adminpb "github.com/wujunhui99/agents_im/service/admin/rpc/admin"
	"github.com/wujunhui99/agents_im/service/msg/api/internal/svc"
	"github.com/wujunhui99/agents_im/service/msg/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/protobuf/types/known/structpb"
)

type CreateFeedbackLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateFeedbackLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateFeedbackLogic {
	return &CreateFeedbackLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreateFeedbackLogic) CreateFeedback(req *types.CreateFeedbackReq) (*types.CreateFeedbackResp, error) {
	userID, err := ctxuser.UserID(l.ctx)
	if err != nil {
		return nil, err
	}
	var clientMeta *structpb.Struct
	if len(req.ClientMeta) > 0 {
		st, serr := structpb.NewStruct(req.ClientMeta)
		if serr != nil {
			return nil, apperror.InvalidArgument("feedback client_meta is invalid")
		}
		clientMeta = st
	}
	result, err := l.svcCtx.AdminRPC.CreateFeedback(l.ctx, &adminpb.FeedbackCreateRequest{
		UserId:     userID,
		Category:   req.Category,
		Title:      req.Title,
		Content:    req.Content,
		Contact:    req.Contact,
		PageUrl:    req.PageURL,
		UserAgent:  req.UserAgent,
		ClientMeta: clientMeta,
	})
	if err != nil {
		return nil, rpcerror.FromStatus(err)
	}
	return &types.CreateFeedbackResp{
		Code:    string(apperror.CodeOK),
		Message: "ok",
		Data: types.FeedbackData{
			FeedbackID: result.GetFeedbackId(),
			Status:     result.GetStatus(),
		},
	}, nil
}
