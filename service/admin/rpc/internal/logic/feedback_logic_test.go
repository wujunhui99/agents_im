package logic

import (
	"context"
	"testing"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/pkg/rpcerror"
	"github.com/wujunhui99/agents_im/service/admin/rpc/admin"
	"github.com/wujunhui99/agents_im/service/admin/rpc/internal/feedbackstore"
	"github.com/wujunhui99/agents_im/service/admin/rpc/internal/svc"
)

func codeOf(t *testing.T, err error) apperror.Code {
	t.Helper()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	return apperror.From(rpcerror.FromStatus(err)).Code
}

// TestCreateFeedbackStoresAuthenticatedUserAndValidates 复刻原 internal/logic.FeedbackLogic 单测：
// 校验规则（user_id/category/title/content）+ 默认 status=new + client_meta 透传。
func TestCreateFeedbackStoresAuthenticatedUserAndValidates(t *testing.T) {
	ctx := context.Background()
	store := feedbackstore.NewMemoryStore()
	svcCtx := &svc.ServiceContext{Feedback: store}
	l := NewCreateFeedbackLogic(ctx, svcCtx)

	resp, err := l.CreateFeedback(&admin.FeedbackCreateRequest{
		UserId:   "1001",
		Category: "bug",
		Title:    "  crash on send  ",
		Content:  "app crashes",
	})
	if err != nil {
		t.Fatalf("create feedback: %v", err)
	}
	if resp.GetFeedbackId() == "" || resp.GetStatus() != "new" {
		t.Fatalf("unexpected create response: %+v", resp)
	}

	stored, err := store.GetFeedback(ctx, resp.GetFeedbackId())
	if err != nil {
		t.Fatalf("get stored feedback: %v", err)
	}
	if stored.UserID != "1001" || stored.Title != "crash on send" {
		t.Fatalf("stored feedback mismatch: %+v", stored)
	}

	if _, err := l.CreateFeedback(&admin.FeedbackCreateRequest{UserId: "1001", Category: "invalid", Title: "x", Content: "y"}); codeOf(t, err) != apperror.CodeInvalidArgument {
		t.Fatalf("expected invalid category to fail with InvalidArgument, got %v", err)
	}
	if _, err := l.CreateFeedback(&admin.FeedbackCreateRequest{UserId: "1001", Category: "bug", Title: " ", Content: "y"}); codeOf(t, err) != apperror.CodeInvalidArgument {
		t.Fatalf("expected blank title to fail with InvalidArgument, got %v", err)
	}
	if _, err := l.CreateFeedback(&admin.FeedbackCreateRequest{UserId: " ", Category: "bug", Title: "x", Content: "y"}); codeOf(t, err) != apperror.CodeInvalidArgument {
		t.Fatalf("expected blank user_id to fail with InvalidArgument, got %v", err)
	}
}

// TestListAndUpdateFeedback 验证 admin triage 读/改路径：状态过滤 + admin_note 更新。
func TestListAndUpdateFeedback(t *testing.T) {
	ctx := context.Background()
	store := feedbackstore.NewMemoryStore()
	svcCtx := &svc.ServiceContext{Feedback: store}

	created, err := NewCreateFeedbackLogic(ctx, svcCtx).CreateFeedback(&admin.FeedbackCreateRequest{
		UserId: "1001", Category: "feature_request", Title: "dark mode", Content: "please",
	})
	if err != nil {
		t.Fatalf("seed feedback: %v", err)
	}

	listResp, err := NewListFeedbackLogic(ctx, svcCtx).ListFeedback(&admin.FeedbackListRequest{Status: "new"})
	if err != nil {
		t.Fatalf("list feedback: %v", err)
	}
	if len(listResp.GetItems()) != 1 || listResp.GetItems()[0].GetFeedbackId() != created.GetFeedbackId() {
		t.Fatalf("unexpected list result: %+v", listResp.GetItems())
	}

	updResp, err := NewUpdateFeedbackLogic(ctx, svcCtx).UpdateFeedback(&admin.FeedbackUpdateRequest{
		FeedbackId: created.GetFeedbackId(), Status: "triaged", AdminNote: "queued",
	})
	if err != nil {
		t.Fatalf("update feedback: %v", err)
	}
	if updResp.GetFeedback().GetStatus() != "triaged" || updResp.GetFeedback().GetAdminNote() != "queued" {
		t.Fatalf("unexpected update result: %+v", updResp.GetFeedback())
	}
}
