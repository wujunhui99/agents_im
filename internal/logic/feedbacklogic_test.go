package logic

import (
	"context"
	"testing"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/internal/model"
	"github.com/wujunhui99/agents_im/internal/repository"
)

func TestFeedbackLogicCreateStoresAuthenticatedUserAndValidatesCategory(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewMemoryFeedbackRepository()
	logic := NewFeedbackLogic(repo)

	created, err := logic.CreateFeedback(ctx, CreateFeedbackRequest{
		UserID:    "usr_1",
		Category:  "bug",
		Title:     "Broken send button",
		Content:   "Clicking send does nothing",
		Contact:   "me@example.com",
		PageURL:   "https://app.example/messages",
		UserAgent: "Mozilla/5.0",
		ClientMeta: map[string]any{
			"viewport": "390x844",
		},
	})
	if err != nil {
		t.Fatalf("CreateFeedback returned error: %v", err)
	}
	if created.FeedbackID == "" {
		t.Fatalf("expected generated feedback id")
	}
	if created.UserID != "usr_1" || created.Category != model.FeedbackCategoryBug || created.Status != model.FeedbackStatusNew {
		t.Fatalf("unexpected feedback persisted: %+v", created)
	}

	if _, err := logic.CreateFeedback(ctx, CreateFeedbackRequest{UserID: "usr_1", Category: "invalid", Title: "x", Content: "y"}); apperror.From(err).Code != apperror.CodeInvalidArgument {
		t.Fatalf("invalid category error = %v, want invalid argument", err)
	}
	if _, err := logic.CreateFeedback(ctx, CreateFeedbackRequest{UserID: "usr_1", Category: "bug", Title: " ", Content: "y"}); apperror.From(err).Code != apperror.CodeInvalidArgument {
		t.Fatalf("empty title error = %v, want invalid argument", err)
	}
}

func TestAdminLogicFeedbackListDetailAndUpdate(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewMemoryFeedbackRepository()
	feedbackLogic := NewFeedbackLogic(repo)
	first, err := feedbackLogic.CreateFeedback(ctx, CreateFeedbackRequest{UserID: "usr_1", Category: "bug", Title: "one", Content: "content one"})
	if err != nil {
		t.Fatalf("create first: %v", err)
	}
	second, err := feedbackLogic.CreateFeedback(ctx, CreateFeedbackRequest{UserID: "usr_2", Category: "feature_request", Title: "two", Content: "content two"})
	if err != nil {
		t.Fatalf("create second: %v", err)
	}

	admin := NewAdminLogic(AdminLogicConfig{Feedback: repo})
	listed, err := admin.ListFeedback(ctx, AdminFeedbackListRequest{Status: "new", Limit: 10})
	if err != nil {
		t.Fatalf("ListFeedback returned error: %v", err)
	}
	if len(listed.Items) != 2 || listed.Items[0].FeedbackID != second.FeedbackID || listed.Items[1].FeedbackID != first.FeedbackID {
		t.Fatalf("unexpected list order/items: %+v", listed.Items)
	}

	detail, err := admin.GetFeedback(ctx, AdminFeedbackDetailRequest{FeedbackID: first.FeedbackID})
	if err != nil {
		t.Fatalf("GetFeedback returned error: %v", err)
	}
	if detail.Feedback.Content != "content one" {
		t.Fatalf("unexpected detail: %+v", detail.Feedback)
	}

	updated, err := admin.UpdateFeedback(ctx, AdminFeedbackUpdateRequest{FeedbackID: first.FeedbackID, Status: "triaged", AdminNote: "known issue"})
	if err != nil {
		t.Fatalf("UpdateFeedback returned error: %v", err)
	}
	if updated.Feedback.Status != string(model.FeedbackStatusTriaged) || updated.Feedback.AdminNote != "known issue" {
		t.Fatalf("unexpected updated feedback: %+v", updated.Feedback)
	}
	if _, err := admin.UpdateFeedback(ctx, AdminFeedbackUpdateRequest{FeedbackID: first.FeedbackID, Status: "invalid"}); apperror.From(err).Code != apperror.CodeInvalidArgument {
		t.Fatalf("invalid status error = %v, want invalid argument", err)
	}
}
