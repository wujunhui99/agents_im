//go:build integration

package convhosting_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/service/agent/rpc/internal/convhosting"
)

// TestPostgresConversationAIHostingStore 验证 agent 域 conversation_ai_hosting_settings 的
// goctl model store（AG-6 ①）对齐旧 internal/repository 语义：ON CONFLICT 代理 PK upsert、
// partial unique index 的“单边互斥”冲突翻译、enabled 行查询。需已迁移到 024 的 PG。
func TestPostgresConversationAIHostingStore(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = os.Getenv("AGENTS_IM_POSTGRES_DSN")
	}
	if dsn == "" {
		t.Skip("DATABASE_URL or AGENTS_IM_POSTGRES_DSN is required for convhosting integration tests")
	}

	ctx := context.Background()
	store := convhosting.NewModelStore(dsn)

	uniq := time.Now().UnixNano()
	ownerA := fmt.Sprintf("usr_chit_a_%d", uniq)
	ownerB := fmt.Sprintf("usr_chit_b_%d", uniq)
	conversationID := fmt.Sprintf("single:%s:%s", ownerA, ownerB)

	// 缺失行 → NotFound（owner 视角与 enabled 视角）。
	if _, err := store.GetConversationAIHostingSetting(ctx, ownerA, conversationID); apperror.From(err).Code != apperror.CodeNotFound {
		t.Fatalf("missing setting error = %v, want not found", err)
	}
	if _, err := store.GetEnabledConversationAIHosting(ctx, conversationID); apperror.From(err).Code != apperror.CodeNotFound {
		t.Fatalf("missing enabled error = %v, want not found", err)
	}

	// owner A 开启 → upsert 返回行；enabled 查询命中 A。
	enabled, err := store.SetConversationAIHostingEnabled(ctx, convhosting.Update{
		OwnerAccountID:    ownerA,
		ConversationID:    conversationID,
		Enabled:           true,
		MaxRecentMessages: 30,
	})
	if err != nil {
		t.Fatalf("enable owner A: %v", err)
	}
	if !enabled.Enabled || enabled.OwnerAccountID != ownerA || enabled.MaxRecentMessages != 30 || enabled.Mode != "auto_reply" {
		t.Fatalf("enabled setting mismatch: %+v", enabled)
	}
	if hit, err := store.GetEnabledConversationAIHosting(ctx, conversationID); err != nil || hit.OwnerAccountID != ownerA {
		t.Fatalf("enabled lookup = %+v err=%v, want owner A", hit, err)
	}

	// owner B 开启 → partial unique index 冲突 → AlreadyExists。
	if _, err := store.SetConversationAIHostingEnabled(ctx, convhosting.Update{
		OwnerAccountID:    ownerB,
		ConversationID:    conversationID,
		Enabled:           true,
		MaxRecentMessages: 30,
	}); apperror.From(err).Code != apperror.CodeAlreadyExists {
		t.Fatalf("peer enable error = %v, want already exists", err)
	}

	// owner A 关闭后 owner B 可开启（upsert 同行更新 + 新行插入交接）。
	if _, err := store.SetConversationAIHostingEnabled(ctx, convhosting.Update{
		OwnerAccountID: ownerA,
		ConversationID: conversationID,
		Enabled:        false,
	}); err != nil {
		t.Fatalf("disable owner A: %v", err)
	}
	if _, err := store.SetConversationAIHostingEnabled(ctx, convhosting.Update{
		OwnerAccountID:    ownerB,
		ConversationID:    conversationID,
		Enabled:           true,
		MaxRecentMessages: 30,
	}); err != nil {
		t.Fatalf("enable owner B after A disabled: %v", err)
	}
	if hit, err := store.GetEnabledConversationAIHosting(ctx, conversationID); err != nil || hit.OwnerAccountID != ownerB {
		t.Fatalf("enabled lookup after handoff = %+v err=%v, want owner B", hit, err)
	}
}
