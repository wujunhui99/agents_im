package logic

import (
	"context"
	"testing"

	"github.com/wujunhui99/agents_im/internal/repository"
)

func TestGetUsersByIDs(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewMemoryRepository()
	userLogic := NewUserLogic(repo)

	alice, err := userLogic.CreateUser(ctx, CreateUserRequest{Identifier: "alice", DisplayName: "Alice"})
	if err != nil {
		t.Fatalf("create alice: %v", err)
	}
	bob, err := userLogic.CreateUser(ctx, CreateUserRequest{Identifier: "bob", DisplayName: "Bob"})
	if err != nil {
		t.Fatalf("create bob: %v", err)
	}

	t.Run("dedup and skip missing", func(t *testing.T) {
		// 重复 id + 空串 + 不存在的 id：去重后只查存在的，缺失静默跳过。
		profiles, err := userLogic.GetUsersByIDs(ctx, GetUsersByIDsRequest{
			UserIDs: []string{alice.UserID, bob.UserID, alice.UserID, "", " ", "missing-id"},
		})
		if err != nil {
			t.Fatalf("GetUsersByIDs: %v", err)
		}
		if len(profiles) != 2 {
			t.Fatalf("want 2 profiles, got %d", len(profiles))
		}
		got := map[string]string{}
		for _, p := range profiles {
			got[p.UserID] = p.Identifier
		}
		if got[alice.UserID] != "alice" || got[bob.UserID] != "bob" {
			t.Fatalf("unexpected profiles: %v", got)
		}
	})

	t.Run("empty input returns nil", func(t *testing.T) {
		profiles, err := userLogic.GetUsersByIDs(ctx, GetUsersByIDsRequest{UserIDs: []string{"", "  "}})
		if err != nil {
			t.Fatalf("GetUsersByIDs empty: %v", err)
		}
		if len(profiles) != 0 {
			t.Fatalf("want 0 profiles, got %d", len(profiles))
		}
	})
}
