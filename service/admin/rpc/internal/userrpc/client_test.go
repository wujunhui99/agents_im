package userrpc

import (
	"context"
	"testing"

	"github.com/wujunhui99/agents_im/common/share/rpcerror"
	"github.com/wujunhui99/agents_im/internal/repository"
	"github.com/wujunhui99/agents_im/service/user/rpc/userclient"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// fakeUser 实现 userclient.User（只覆盖适配器用到的方法，其余靠内嵌接口、未调用）。
type fakeUser struct {
	userclient.User
	getResp    *userclient.UserResponse
	searchResp *userclient.SearchAccountsResponse
	countResp  *userclient.CountAccountsResponse
	err        error
	lastSearch *userclient.SearchAccountsRequest
}

func (f *fakeUser) GetUserByID(_ context.Context, _ *userclient.GetUserByIDRequest, _ ...grpc.CallOption) (*userclient.UserResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.getResp, nil
}

func (f *fakeUser) SearchAccounts(_ context.Context, in *userclient.SearchAccountsRequest, _ ...grpc.CallOption) (*userclient.SearchAccountsResponse, error) {
	f.lastSearch = in
	if f.err != nil {
		return nil, f.err
	}
	return f.searchResp, nil
}

func (f *fakeUser) CountAccounts(_ context.Context, _ *userclient.CountAccountsRequest, _ ...grpc.CallOption) (*userclient.CountAccountsResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.countResp, nil
}

func TestGetByIDMapsAvatarAndProfile(t *testing.T) {
	fake := &fakeUser{getResp: &userclient.UserResponse{User: &userclient.UserEntity{
		UserId:        "100",
		Identifier:    "alice",
		DisplayName:   "Alice",
		AccountType:   "user",
		AvatarMediaId: "323130844539310080",
		AvatarUrl:     "/media/avatars/323130844539310080",
	}}}
	user, err := NewAdminAccountClient(fake).GetByID(context.Background(), "100")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if user.AccountID != "100" || user.DisplayName != "Alice" {
		t.Fatalf("mapped user wrong: %+v", user)
	}
	if user.AvatarMediaID != "323130844539310080" || user.AvatarURL != "/media/avatars/323130844539310080" {
		t.Fatalf("avatar not mapped: media=%q url=%q", user.AvatarMediaID, user.AvatarURL)
	}
}

func TestSearchAccountsPassesFilterAndMaps(t *testing.T) {
	fake := &fakeUser{searchResp: &userclient.SearchAccountsResponse{Users: []*userclient.UserEntity{
		{UserId: "1", Identifier: "alice", AvatarMediaId: "m1"},
		{UserId: "2", Identifier: "alicia"},
	}}}
	users, err := NewAdminAccountClient(fake).SearchAccounts(context.Background(), repository.AccountSearchFilter{Query: "ali", Limit: 7})
	if err != nil {
		t.Fatalf("SearchAccounts: %v", err)
	}
	if fake.lastSearch.GetQuery() != "ali" || fake.lastSearch.GetLimit() != 7 {
		t.Fatalf("filter not forwarded: %+v", fake.lastSearch)
	}
	if len(users) != 2 || users[0].AvatarMediaID != "m1" {
		t.Fatalf("mapped users wrong: %+v", users)
	}
}

func TestCountAccountsMaps(t *testing.T) {
	fake := &fakeUser{countResp: &userclient.CountAccountsResponse{Count: 42}}
	count, err := NewAdminAccountClient(fake).CountAccounts(context.Background())
	if err != nil {
		t.Fatalf("CountAccounts: %v", err)
	}
	if count != 42 {
		t.Fatalf("count = %d, want 42", count)
	}
}

// gRPC status 错误经 rpcerror.FromStatus 还原成 apperror，再 ToStatus 应得回原码。
func TestGetByIDPropagatesNotFoundCode(t *testing.T) {
	fake := &fakeUser{err: status.Error(codes.NotFound, "account not found")}
	_, err := NewAdminAccountClient(fake).GetByID(context.Background(), "999")
	if err == nil {
		t.Fatal("expected error")
	}
	if st, _ := status.FromError(rpcerror.ToStatus(err)); st.Code() != codes.NotFound {
		t.Fatalf("round-trip code = %s, want NotFound", st.Code())
	}
}
