package logic

import (
	"context"
	"testing"

	"github.com/wujunhui99/agents_im/service/user/rpc/internal/model"
	userpb "github.com/wujunhui99/agents_im/service/user/rpc/user"
)

func TestCreateTestAccountCreatesTestTypeAndProvisionsAssistant(t *testing.T) {
	ctx := context.Background()
	store := newFakeStore()
	prov := &fakeProvisioner{}
	svcCtx := newTestSvc(store, prov, &fakeAvatarValidator{})

	resp, err := NewCreateTestAccountLogic(ctx, svcCtx).CreateTestAccount(&userpb.CreateTestAccountRequest{
		Identifier:  "Test_User1",
		DisplayName: "测试账户一号",
	})
	if err != nil {
		t.Fatalf("CreateTestAccount: %v", err)
	}
	if resp.GetAlreadyExists() {
		t.Fatal("first create should not report already_exists")
	}
	user := resp.GetUser()
	if got := user.GetIdentifier(); got != "test_user1" {
		t.Fatalf("identifier = %q, want lowercased test_user1", got)
	}
	if got := user.GetAccountType(); got != "test" {
		t.Fatalf("account_type = %q, want test", got)
	}
	if got := user.GetEmail(); got != "" {
		t.Fatalf("email = %q, want empty (test accounts bind no email)", got)
	}
	if len(prov.ensuredFor) != 1 || prov.ensuredFor[0] != user.GetUserId() {
		t.Fatalf("expected assistant provisioned once for test account, got %v", prov.ensuredFor)
	}
	if rec := store.byID[user.GetUserId()]; rec == nil || rec.AccountType != model.AccountTypeTest {
		t.Fatalf("stored account_type should be %d (test)", model.AccountTypeTest)
	}
}

func TestCreateTestAccountIsIdempotentForExistingTestAccount(t *testing.T) {
	ctx := context.Background()
	store := newFakeStore()
	prov := &fakeProvisioner{}
	svcCtx := newTestSvc(store, prov, &fakeAvatarValidator{})

	first, err := NewCreateTestAccountLogic(ctx, svcCtx).CreateTestAccount(&userpb.CreateTestAccountRequest{Identifier: "tester"})
	if err != nil {
		t.Fatalf("first CreateTestAccount: %v", err)
	}
	second, err := NewCreateTestAccountLogic(ctx, svcCtx).CreateTestAccount(&userpb.CreateTestAccountRequest{Identifier: "tester"})
	if err != nil {
		t.Fatalf("second CreateTestAccount: %v", err)
	}
	if !second.GetAlreadyExists() {
		t.Fatal("second create should report already_exists")
	}
	if first.GetUser().GetUserId() != second.GetUser().GetUserId() {
		t.Fatal("second create should return the same account")
	}
	if len(prov.ensuredFor) != 1 {
		t.Fatalf("assistant should be provisioned only on first create, got %v", prov.ensuredFor)
	}
}

func TestCreateTestAccountRejectsNonTestIdentifierConflict(t *testing.T) {
	ctx := context.Background()
	store := newFakeStore()
	prov := &fakeProvisioner{}
	svcCtx := newTestSvc(store, prov, &fakeAvatarValidator{})

	if _, err := NewCreateUserLogic(ctx, svcCtx).CreateUser(&userpb.CreateUserRequest{
		Identifier:  "normal_user",
		DisplayName: "普通用户",
		AccountType: "user",
	}); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if _, err := NewCreateTestAccountLogic(ctx, svcCtx).CreateTestAccount(&userpb.CreateTestAccountRequest{Identifier: "normal_user"}); err == nil {
		t.Fatal("creating a test account over an existing user identifier should fail")
	}
}

func TestCreateTestAccountDefaultsDisplayNameToIdentifier(t *testing.T) {
	ctx := context.Background()
	store := newFakeStore()
	svcCtx := newTestSvc(store, &fakeProvisioner{}, &fakeAvatarValidator{})

	resp, err := NewCreateTestAccountLogic(ctx, svcCtx).CreateTestAccount(&userpb.CreateTestAccountRequest{Identifier: "tester2"})
	if err != nil {
		t.Fatalf("CreateTestAccount: %v", err)
	}
	if got := resp.GetUser().GetDisplayName(); got != "tester2" {
		t.Fatalf("display_name = %q, want fallback to identifier", got)
	}
}
