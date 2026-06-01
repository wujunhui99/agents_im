package logic

import (
	"context"
	"testing"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/common/share/model"
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

