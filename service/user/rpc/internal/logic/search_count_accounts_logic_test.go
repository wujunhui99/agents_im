package logic

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/wujunhui99/agents_im/service/user/rpc/internal/model"
	userpb "github.com/wujunhui99/agents_im/service/user/rpc/user"
)

func TestSearchAccountsMatchesQueryAcrossColumns(t *testing.T) {
	ctx := context.Background()
	store := newFakeStore()
	store.byID["a1"] = &model.AccountProfile{AccountID: "a1", Identifier: "alice", DisplayName: "Alice A", Name: "Alice"}
	store.byID["b2"] = &model.AccountProfile{AccountID: "b2", Identifier: "bob", DisplayName: "Bob B", Name: "Bob"}
	store.byID["c3"] = &model.AccountProfile{AccountID: "c3", Identifier: "carol", DisplayName: "Alicia", Name: "Carol"}
	svcCtx := newTestSvc(store, &fakeProvisioner{}, &fakeAvatarValidator{})

	// 大小写不敏感，命中 display_name 的 "Alic" -> Alice A + Alicia。
	resp, err := NewSearchAccountsLogic(ctx, svcCtx).SearchAccounts(&userpb.SearchAccountsRequest{Query: "ALIC"})
	if err != nil {
		t.Fatalf("SearchAccounts: %v", err)
	}
	got := map[string]bool{}
	for _, u := range resp.GetUsers() {
		got[u.GetUserId()] = true
	}
	if len(got) != 2 || !got["a1"] || !got["c3"] {
		t.Fatalf("expected {a1,c3} matching Alic, got %v", got)
	}
}

func TestSearchAccountsEmptyQueryReturnsAllUpToDefaultLimit(t *testing.T) {
	ctx := context.Background()
	store := newFakeStore()
	// 25 条 -> 空 query + limit=0 应被 normalize 成默认 20。
	for i := 0; i < 25; i++ {
		id := "acct" + strconv.Itoa(i)
		store.byID[id] = &model.AccountProfile{AccountID: id, Identifier: id}
	}
	svcCtx := newTestSvc(store, &fakeProvisioner{}, &fakeAvatarValidator{})

	resp, err := NewSearchAccountsLogic(ctx, svcCtx).SearchAccounts(&userpb.SearchAccountsRequest{})
	if err != nil {
		t.Fatalf("SearchAccounts: %v", err)
	}
	if len(resp.GetUsers()) != defaultSearchAccountsLimit {
		t.Fatalf("empty query default limit = %d, want %d", len(resp.GetUsers()), defaultSearchAccountsLimit)
	}
}

func TestSearchAccountsRespectsExplicitLimit(t *testing.T) {
	ctx := context.Background()
	store := newFakeStore()
	for i := 0; i < 5; i++ {
		id := "acct" + strconv.Itoa(i)
		store.byID[id] = &model.AccountProfile{AccountID: id, Identifier: id}
	}
	svcCtx := newTestSvc(store, &fakeProvisioner{}, &fakeAvatarValidator{})

	resp, err := NewSearchAccountsLogic(ctx, svcCtx).SearchAccounts(&userpb.SearchAccountsRequest{Limit: 2})
	if err != nil {
		t.Fatalf("SearchAccounts: %v", err)
	}
	if len(resp.GetUsers()) != 2 {
		t.Fatalf("explicit limit=2 returned %d", len(resp.GetUsers()))
	}
}

func TestSearchAccountsOrdersByCreatedAtDesc(t *testing.T) {
	ctx := context.Background()
	store := newFakeStore()
	store.byID["old"] = &model.AccountProfile{AccountID: "old", Identifier: "old", AccountCreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	store.byID["new"] = &model.AccountProfile{AccountID: "new", Identifier: "new", AccountCreatedAt: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)}
	svcCtx := newTestSvc(store, &fakeProvisioner{}, &fakeAvatarValidator{})

	resp, err := NewSearchAccountsLogic(ctx, svcCtx).SearchAccounts(&userpb.SearchAccountsRequest{})
	if err != nil {
		t.Fatalf("SearchAccounts: %v", err)
	}
	if len(resp.GetUsers()) != 2 || resp.GetUsers()[0].GetUserId() != "new" {
		t.Fatalf("expected newest first, got %+v", resp.GetUsers())
	}
}

func TestCountAccounts(t *testing.T) {
	ctx := context.Background()
	store := newFakeStore()
	store.byID["a"] = &model.AccountProfile{AccountID: "a", Identifier: "a"}
	store.byID["b"] = &model.AccountProfile{AccountID: "b", Identifier: "b"}
	svcCtx := newTestSvc(store, &fakeProvisioner{}, &fakeAvatarValidator{})

	resp, err := NewCountAccountsLogic(ctx, svcCtx).CountAccounts(&userpb.CountAccountsRequest{})
	if err != nil {
		t.Fatalf("CountAccounts: %v", err)
	}
	if resp.GetCount() != 2 {
		t.Fatalf("count = %d, want 2", resp.GetCount())
	}
}
