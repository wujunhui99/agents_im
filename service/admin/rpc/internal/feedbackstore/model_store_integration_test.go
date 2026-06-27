//go:build integration

package feedbackstore_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/pkg/model"
	"github.com/wujunhui99/agents_im/service/admin/rpc/internal/feedbackstore"
)

// TestPostgresFeedbackStore 验证 feedback 表的 goctl model store（#678）对齐旧
// internal/repository.PostgresFeedbackRepository 语义：feedback_id bigint keystone `::text` 读写、
// client_meta jsonb 编解码、status 过滤分页、Update 只改 status/admin_note、不存在返回 NotFound。
// 需已迁移 015 的 PG（feedback.user_id FK → accounts.account_id，测试自行 seed 账号）。
func TestPostgresFeedbackStore(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = os.Getenv("AGENTS_IM_POSTGRES_DSN")
	}
	if dsn == "" {
		t.Skip("DATABASE_URL or AGENTS_IM_POSTGRES_DSN is required for feedbackstore integration tests")
	}

	ctx := context.Background()
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	base := time.Now().UnixNano() % 1_000_000_000
	userID := fmt.Sprintf("acct_fb_%d", base)
	// feedback_id 为 bigint keystone，必须是数字串。
	feedbackID := fmt.Sprintf("%d", 600_000_000_000+base)

	// seed 账号（feedback.user_id FK），结束清理（feedback 经 ON DELETE CASCADE 一并清掉）。
	if _, err := db.ExecContext(ctx,
		`insert into accounts (account_id, identifier, email_normalized) values ($1, $2, '')`,
		userID, fmt.Sprintf("fb-int-%d", base)); err != nil {
		t.Fatalf("seed account: %v", err)
	}
	defer db.ExecContext(ctx, `delete from accounts where account_id = $1`, userID)

	store := feedbackstore.NewModelStore(dsn)

	created, err := store.CreateFeedback(ctx, model.Feedback{
		FeedbackID: feedbackID,
		UserID:     userID,
		Category:   model.FeedbackCategoryBug,
		Title:      "crash on send",
		Content:    "app crashes when sending",
		Contact:    "me@example.com",
		ClientMeta: map[string]any{"build": "1.2.3"},
	})
	if err != nil {
		t.Fatalf("create feedback: %v", err)
	}
	if created.FeedbackID != feedbackID || created.Status != model.FeedbackStatusNew {
		t.Fatalf("unexpected created feedback: %+v", created)
	}
	if created.ClientMeta["build"] != "1.2.3" {
		t.Fatalf("client_meta roundtrip mismatch: %+v", created.ClientMeta)
	}

	got, err := store.GetFeedback(ctx, feedbackID)
	if err != nil {
		t.Fatalf("get feedback: %v", err)
	}
	if got.UserID != userID || got.Contact != "me@example.com" {
		t.Fatalf("unexpected get feedback: %+v", got)
	}

	listed, err := store.ListFeedback(ctx, feedbackstore.ListFilter{Status: model.FeedbackStatusNew, Limit: 100})
	if err != nil {
		t.Fatalf("list feedback: %v", err)
	}
	found := false
	for _, fb := range listed {
		if fb.FeedbackID == feedbackID {
			found = true
		}
	}
	if !found {
		t.Fatalf("created feedback not in new-status list (%d items)", len(listed))
	}

	updated, err := store.UpdateFeedback(ctx, model.Feedback{
		FeedbackID: feedbackID,
		Status:     model.FeedbackStatusTriaged,
		AdminNote:  "queued for next sprint",
	})
	if err != nil {
		t.Fatalf("update feedback: %v", err)
	}
	if updated.Status != model.FeedbackStatusTriaged || updated.AdminNote != "queued for next sprint" {
		t.Fatalf("unexpected updated feedback: %+v", updated)
	}

	if _, err := store.GetFeedback(ctx, "599999999999"); apperror.From(err).Code != apperror.CodeNotFound {
		t.Fatalf("expected NotFound for missing feedback, got %v", err)
	}
}
