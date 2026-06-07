package logic

import (
	"context"
	"testing"

	"github.com/wujunhui99/agents_im/service/user/rpc/internal/model"
	"github.com/wujunhui99/agents_im/service/user/rpc/internal/svc"
	userpb "github.com/wujunhui99/agents_im/service/user/rpc/user"
)

func newTestSvc(store *fakeStore, prov *fakeProvisioner, val *fakeAvatarValidator) *svc.ServiceContext {
	return &svc.ServiceContext{
		Accounts:        newFakeAccountsModel(store),
		Profiles:        newFakeProfilesModel(store),
		Assistant:       prov,
		AvatarValidator: val,
	}
}

func TestCreateUserPersistsAndProvisionsAssistantForUser(t *testing.T) {
	ctx := context.Background()
	store := newFakeStore()
	prov := &fakeProvisioner{}
	svcCtx := newTestSvc(store, prov, &fakeAvatarValidator{})

	resp, err := NewCreateUserLogic(ctx, svcCtx).CreateUser(&userpb.CreateUserRequest{
		Identifier:  "Rpc_User", // 应被小写化为唯一键
		DisplayName: "RPC User",
		Gender:      "male",
		AccountType: "user",
		Email:       " a@b.com ",
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if got := resp.GetUser().GetIdentifier(); got != "rpc_user" {
		t.Fatalf("identifier = %q, want lowercased rpc_user", got)
	}
	if got := resp.GetUser().GetName(); got != "RPC User" {
		t.Fatalf("name = %q, want mirrored from display_name", got)
	}
	if got := resp.GetUser().GetEmail(); got != "a@b.com" {
		t.Fatalf("email = %q, want trimmed", got)
	}
	if len(prov.ensuredFor) != 1 || prov.ensuredFor[0] != resp.GetUser().GetUserId() {
		t.Fatalf("expected assistant provisioned once for new user, got %v", prov.ensuredFor)
	}
}

func TestCreateUserSkipsAssistantForNonUser(t *testing.T) {
	ctx := context.Background()
	store := newFakeStore()
	prov := &fakeProvisioner{}
	svcCtx := newTestSvc(store, prov, &fakeAvatarValidator{})

	if _, err := NewCreateUserLogic(ctx, svcCtx).CreateUser(&userpb.CreateUserRequest{
		Identifier:  "agent_acct",
		DisplayName: "Agent",
		AccountType: "agent",
	}); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if len(prov.ensuredFor) != 0 {
		t.Fatalf("expected no assistant provisioning for agent account, got %v", prov.ensuredFor)
	}
}

func TestUpdateUserAvatarValidatesThenUpdates(t *testing.T) {
	ctx := context.Background()
	store := newFakeStore()
	store.byID["acct1"] = &model.AccountProfile{AccountID: "acct1", Identifier: "acct1"}
	val := &fakeAvatarValidator{}
	svcCtx := newTestSvc(store, &fakeProvisioner{}, val)

	resp, err := NewUpdateUserAvatarLogic(ctx, svcCtx).UpdateUserAvatar(&userpb.UpdateUserAvatarRequest{
		UserId:        "acct1",
		AvatarMediaId: "med_x",
	})
	if err != nil {
		t.Fatalf("UpdateUserAvatar: %v", err)
	}
	if val.calls != 1 {
		t.Fatalf("expected avatar validator called once, got %d", val.calls)
	}
	if got := resp.GetUser().GetAvatarMediaId(); got != "med_x" {
		t.Fatalf("avatar_media_id = %q", got)
	}
	if got := resp.GetUser().GetAvatarUrl(); got != "/media/avatars/med_x" {
		t.Fatalf("avatar_url = %q", got)
	}
}

func TestUpdateUserProfileMirrorsDisplayNameToName(t *testing.T) {
	ctx := context.Background()
	store := newFakeStore()
	store.byID["acct1"] = &model.AccountProfile{AccountID: "acct1", Identifier: "acct1", DisplayName: "old", Name: "old"}
	svcCtx := newTestSvc(store, &fakeProvisioner{}, &fakeAvatarValidator{})

	display := "New Name"
	resp, err := NewUpdateUserProfileLogic(ctx, svcCtx).UpdateUserProfile(&userpb.UpdateUserProfileRequest{
		UserId:      "acct1",
		DisplayName: &display,
	})
	if err != nil {
		t.Fatalf("UpdateUserProfile: %v", err)
	}
	if resp.GetUser().GetDisplayName() != "New Name" || resp.GetUser().GetName() != "New Name" {
		t.Fatalf("expected display_name+name mirrored, got display=%q name=%q",
			resp.GetUser().GetDisplayName(), resp.GetUser().GetName())
	}
}

func TestGetUsersByIDsReturnsFoundSubset(t *testing.T) {
	ctx := context.Background()
	store := newFakeStore()
	store.byID["a"] = &model.AccountProfile{AccountID: "a", Identifier: "a"}
	store.byID["b"] = &model.AccountProfile{AccountID: "b", Identifier: "b"}
	svcCtx := newTestSvc(store, &fakeProvisioner{}, &fakeAvatarValidator{})

	resp, err := NewGetUsersByIDsLogic(ctx, svcCtx).GetUsersByIDs(&userpb.GetUsersByIDsRequest{
		UserIds: []string{"a", " ", "missing", "a"},
	})
	if err != nil {
		t.Fatalf("GetUsersByIDs: %v", err)
	}
	if len(resp.GetUsers()) != 1 {
		t.Fatalf("expected 1 found (a, deduped; b not requested), got %d", len(resp.GetUsers()))
	}
}
