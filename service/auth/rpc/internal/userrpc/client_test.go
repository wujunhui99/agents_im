package userrpc

import (
	"context"
	"testing"
	"time"

	"github.com/wujunhui99/agents_im/common/share/rpcerror"
	"github.com/wujunhui99/agents_im/internal/auth/useradapter"
	"github.com/wujunhui99/agents_im/service/user/rpc/userclient"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// fakeUser 实现 userclient.User（只覆盖 auth 用到的 3 个方法，其余靠内嵌接口、未调用）。
type fakeUser struct {
	userclient.User
	existsResp *userclient.ExistsByIdentifierResponse
	userResp   *userclient.UserResponse
	err        error
	lastCreate *userclient.CreateUserRequest
}

func (f *fakeUser) ExistsByIdentifier(_ context.Context, _ *userclient.ExistsByIdentifierRequest, _ ...grpc.CallOption) (*userclient.ExistsByIdentifierResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.existsResp, nil
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

func TestCreateUserPassesEmailVerifiedAtRFC3339AndMapsProfile(t *testing.T) {
	verifiedAt := time.Date(2026, 6, 13, 2, 39, 45, 0, time.UTC)
	fake := &fakeUser{userResp: &userclient.UserResponse{User: &userclient.UserEntity{
		UserId:        "323130844539310080",
		Identifier:    "alice",
		Email:         "alice@example.com",
		AccountType:   "user",
		AvatarMediaId: "59109426052726784",
		AvatarUrl:     "/media/avatars/59109426052726784",
	}}}
	profile, err := NewClient(fake).CreateUser(context.Background(), useradapter.CreateUserRequest{
		Identifier:      "alice",
		Email:           "alice@example.com",
		EmailVerifiedAt: verifiedAt,
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if fake.lastCreate.GetEmailVerifiedAt() != "2026-06-13T02:39:45Z" {
		t.Fatalf("email_verified_at wire = %q, want RFC3339", fake.lastCreate.GetEmailVerifiedAt())
	}
	if profile.UserID != "323130844539310080" || profile.AvatarMediaID != "59109426052726784" || profile.AccountType != "user" {
		t.Fatalf("mapped profile wrong: %+v", profile)
	}
}

func TestCreateUserEmptyVerifiedAtSendsEmptyString(t *testing.T) {
	fake := &fakeUser{userResp: &userclient.UserResponse{User: &userclient.UserEntity{UserId: "1"}}}
	if _, err := NewClient(fake).CreateUser(context.Background(), useradapter.CreateUserRequest{Identifier: "bob"}); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if fake.lastCreate.GetEmailVerifiedAt() != "" {
		t.Fatalf("zero verifiedAt should map to empty string, got %q", fake.lastCreate.GetEmailVerifiedAt())
	}
}

func TestExistsByIdentifierMaps(t *testing.T) {
	fake := &fakeUser{existsResp: &userclient.ExistsByIdentifierResponse{Identifier: "alice", Exists: true}}
	res, err := NewClient(fake).ExistsByIdentifier(context.Background(), "alice")
	if err != nil {
		t.Fatalf("ExistsByIdentifier: %v", err)
	}
	if !res.Exists || res.Identifier != "alice" {
		t.Fatalf("unexpected: %+v", res)
	}
}

// gRPC status 错误经 rpcerror.FromStatus 还原成 apperror，再 ToStatus 应得回原码（不被吞成 Unknown）。
func TestGetUserByIDPropagatesNotFoundCode(t *testing.T) {
	fake := &fakeUser{err: status.Error(codes.NotFound, "user not found")}
	_, err := NewClient(fake).GetUserByID(context.Background(), "999")
	if err == nil {
		t.Fatal("expected error")
	}
	if st, _ := status.FromError(rpcerror.ToStatus(err)); st.Code() != codes.NotFound {
		t.Fatalf("round-trip code = %s, want NotFound", st.Code())
	}
}
