package logic

import (
	"context"
	"strings"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/pkg/model"
	"github.com/wujunhui99/agents_im/pkg/rpcerror"
	"github.com/wujunhui99/agents_im/service/admin/rpc/admin"
	"github.com/wujunhui99/agents_im/service/admin/rpc/internal/feedbackstore"
	"github.com/wujunhui99/agents_im/service/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

const adminFeedbackIDMaxLen = 128

// ---- ListFeedback ----

type ListFeedbackLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListFeedbackLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListFeedbackLogic {
	return &ListFeedbackLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *ListFeedbackLogic) ListFeedback(in *admin.FeedbackListRequest) (*admin.FeedbackListResponse, error) {
	if l.svcCtx.Feedback == nil {
		return nil, rpcerror.ToStatus(apperror.Internal("feedback repository is not configured"))
	}
	var status model.FeedbackStatus
	if strings.TrimSpace(in.GetStatus()) != "" {
		parsed, ok := model.NormalizeFeedbackStatus(strings.TrimSpace(in.GetStatus()))
		if !ok {
			return nil, rpcerror.ToStatus(apperror.InvalidArgument("feedback status is invalid"))
		}
		status = parsed
	}
	items, err := l.svcCtx.Feedback.ListFeedback(l.ctx, feedbackstore.ListFilter{
		Status: status,
		Limit:  normalizeAdminLimit(int(in.GetLimit()), 50, 200),
		Offset: int(in.GetOffset()),
	})
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	out := make([]*admin.AdminFeedback, 0, len(items))
	for _, item := range items {
		pb, err := adminFeedbackPB(item)
		if err != nil {
			return nil, rpcerror.ToStatus(err)
		}
		out = append(out, pb)
	}
	return &admin.FeedbackListResponse{Items: out}, nil
}

// ---- GetFeedback ----

type GetFeedbackLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetFeedbackLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetFeedbackLogic {
	return &GetFeedbackLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *GetFeedbackLogic) GetFeedback(in *admin.FeedbackDetailRequest) (*admin.FeedbackDetailResponse, error) {
	if l.svcCtx.Feedback == nil {
		return nil, rpcerror.ToStatus(apperror.Internal("feedback repository is not configured"))
	}
	feedbackID, err := validateRequiredAdminID(in.GetFeedbackId(), "feedback_id", adminFeedbackIDMaxLen)
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	feedback, err := l.svcCtx.Feedback.GetFeedback(l.ctx, feedbackID)
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	pb, err := adminFeedbackPB(feedback)
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	return &admin.FeedbackDetailResponse{Feedback: pb}, nil
}

// ---- UpdateFeedback ----

type UpdateFeedbackLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateFeedbackLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateFeedbackLogic {
	return &UpdateFeedbackLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *UpdateFeedbackLogic) UpdateFeedback(in *admin.FeedbackUpdateRequest) (*admin.FeedbackUpdateResponse, error) {
	if l.svcCtx.Feedback == nil {
		return nil, rpcerror.ToStatus(apperror.Internal("feedback repository is not configured"))
	}
	feedbackID, err := validateRequiredAdminID(in.GetFeedbackId(), "feedback_id", adminFeedbackIDMaxLen)
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	status, ok := model.NormalizeFeedbackStatus(strings.TrimSpace(in.GetStatus()))
	if !ok {
		return nil, rpcerror.ToStatus(apperror.InvalidArgument("feedback status is invalid"))
	}
	updated, err := l.svcCtx.Feedback.UpdateFeedback(l.ctx, model.Feedback{
		FeedbackID: feedbackID,
		Status:     status,
		AdminNote:  strings.TrimSpace(in.GetAdminNote()),
	})
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	pb, err := adminFeedbackPB(updated)
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	return &admin.FeedbackUpdateResponse{Feedback: pb}, nil
}

// ---- CreateFeedback ----
// 用户提交反馈（msg-api BFF 的 POST /api/feedback 经 gRPC 调入）。
// 校验规则对齐原 internal/logic.FeedbackLogic.CreateFeedback。

type CreateFeedbackLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateFeedbackLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateFeedbackLogic {
	return &CreateFeedbackLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *CreateFeedbackLogic) CreateFeedback(in *admin.FeedbackCreateRequest) (*admin.FeedbackCreateResponse, error) {
	if l.svcCtx.Feedback == nil {
		return nil, rpcerror.ToStatus(apperror.Internal("feedback repository is not configured"))
	}
	userID := strings.TrimSpace(in.GetUserId())
	if userID == "" {
		return nil, rpcerror.ToStatus(apperror.InvalidArgument("user_id is required"))
	}
	category, ok := model.NormalizeFeedbackCategory(strings.TrimSpace(in.GetCategory()))
	if !ok {
		return nil, rpcerror.ToStatus(apperror.InvalidArgument("feedback category is invalid"))
	}
	title := strings.TrimSpace(in.GetTitle())
	if title == "" {
		return nil, rpcerror.ToStatus(apperror.InvalidArgument("feedback title is required"))
	}
	content := strings.TrimSpace(in.GetContent())
	if content == "" {
		return nil, rpcerror.ToStatus(apperror.InvalidArgument("feedback content is required"))
	}
	var clientMeta map[string]any
	if in.GetClientMeta() != nil {
		clientMeta = in.GetClientMeta().AsMap()
	}
	created, err := l.svcCtx.Feedback.CreateFeedback(l.ctx, model.Feedback{
		UserID:     userID,
		Category:   category,
		Status:     model.FeedbackStatusNew,
		Title:      title,
		Content:    content,
		Contact:    strings.TrimSpace(in.GetContact()),
		PageURL:    strings.TrimSpace(in.GetPageUrl()),
		UserAgent:  strings.TrimSpace(in.GetUserAgent()),
		ClientMeta: clientMeta,
	})
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	return &admin.FeedbackCreateResponse{
		FeedbackId: created.FeedbackID,
		Status:     string(created.Status),
	}, nil
}
