package userrpc

import (
	"context"
	"testing"

	"github.com/wujunhui99/agents_im/pkg/model"
	"github.com/wujunhui99/agents_im/pkg/rpcerror"
	"github.com/wujunhui99/agents_im/internal/repository"
	"github.com/wujunhui99/agents_im/service/user/rpc/userclient"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// fakeUser 实现 userclient.User（只覆盖适配器用到的方法，其余靠内嵌接口、未调用）。
type fakeUser struct {
	userclient.User
	userResp   *userclient.UserResponse
	err        error
	lastCreate *userclient.CreateUserRequest
}

func (f *fakeUser) CreateUser(_ context.Context, in *userclient.CreateUserRequest, _ ...grpc.CallOption) (*userclient.UserResponse, error) {
	f.lastCreate = in
	if f.err != nil {
		return nil, f.err
	}
	return f.userResp, nil
}

func (f *fakeUser) GetUserByID(_ context.Context, _ *userclient.GetUserByIDRequest, _ ...grpc.CallOption) (*userclient.UserResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.userResp, nil
}

// fakeFriendships 是好友委托桩，只验证 EnsureAcceptedFriendship 被透传调用。
type fakeFriendships struct {
	repository.FriendshipRepository
	ensured [2]string
	called  bool
}

func (f *fakeFriendships) EnsureAcceptedFriendship(_ context.Context, userID, friendID string) error {
	f.called = true
	f.ensured = [2]string{userID, friendID}
	return nil
}

func TestCreatePassesAccountTypeAndMaps(t *testing.T) {
	fake := &fakeUser{userResp: &userclient.UserResponse{User: &userclient.UserEntity{
		UserId:      "323130844539310080",
		Identifier:  "assistant-bot",
		AccountType: string(model.AccountTypeAgent),
	}}}
	c := NewComposite(fake, &fakeFriendships{})
	user, err := c.Create(context.Background(), model.User{
		Identifier:  "assistant-bot",
		DisplayName: "Bot",
		Name:        "Bot",
		AccountType: model.AccountTypeAgent,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if fake.lastCreate.GetAccountType() != string(model.AccountTypeAgent) {
		t.Fatalf("account_type wire = %q, want agent", fake.lastCreate.GetAccountType())
	}
	if user.AccountID != "323130844539310080" || user.AccountType != model.AccountTypeAgent {
		t.Fatalf("mapped user wrong: %+v", user)
	}
}

func TestFriendshipDelegated(t *testing.T) {
	fr := &fakeFriendships{}
	c := NewComposite(&fakeUser{}, fr)
	if err := c.EnsureAcceptedFriendship(context.Background(), "human", "agent"); err != nil {
		t.Fatalf("EnsureAcceptedFriendship: %v", err)
	}
	if !fr.called || fr.ensured != [2]string{"human", "agent"} {
		t.Fatalf("friendship not delegated: %+v", fr)
	}
}

// gRPC status 错误经 rpcerror.FromStatus 还原成 apperror，再 ToStatus 应得回原码。
func TestGetByIDPropagatesNotFoundCode(t *testing.T) {
	fake := &fakeUser{err: status.Error(codes.NotFound, "account not found")}
	_, err := NewComposite(fake, &fakeFriendships{}).GetByID(context.Background(), "999")
	if err == nil {
		t.Fatal("expected error")
	}
	if st, _ := status.FromError(rpcerror.ToStatus(err)); st.Code() != codes.NotFound {
		t.Fatalf("round-trip code = %s, want NotFound", st.Code())
	}
}

// user-rpc 未暴露的账号写在 agent-create 路径不可达：必须 fail-loud(rule 1/2)。
func TestUnsupportedAccountWritesFailLoud(t *testing.T) {
	c := NewComposite(&fakeUser{}, &fakeFriendships{})
	if _, err := c.UpdateAvatar(context.Background(), "1", "2", "/u"); err == nil {
		t.Fatal("UpdateAvatar must fail loud")
	}
	if _, err := c.UpdateProfile(context.Background(), "1", repository.AccountProfilePatch{}); err == nil {
		t.Fatal("UpdateProfile must fail loud")
	}
	if _, err := c.RenameIdentifier(context.Background(), "a", "b"); err == nil {
		t.Fatal("RenameIdentifier must fail loud")
	}
	if _, err := c.ListByAccountType(context.Background(), model.AccountTypeAgent); err == nil {
		t.Fatal("ListByAccountType must fail loud")
	}
}
