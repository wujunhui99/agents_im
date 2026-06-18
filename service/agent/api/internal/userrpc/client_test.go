package userrpc

import (
	"context"
	"testing"

	"github.com/wujunhui99/agents_im/internal/repository"
	"github.com/wujunhui99/agents_im/pkg/model"
	"github.com/wujunhui99/agents_im/pkg/rpcerror"
	"github.com/wujunhui99/agents_im/service/user/rpc/userclient"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// fakeUser 实现 userclient.User（只覆盖适配器用到的只读方法，其余靠内嵌接口、未调用）。
type fakeUser struct {
	userclient.User
	userResp  *userclient.UserResponse
	usersResp *userclient.GetUsersByIDsResponse
	exists    *userclient.ExistsByIdentifierResponse
	err       error
}

func (f *fakeUser) GetUserByID(_ context.Context, _ *userclient.GetUserByIDRequest, _ ...grpc.CallOption) (*userclient.UserResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.userResp, nil
}

func (f *fakeUser) GetUserByIdentifier(_ context.Context, _ *userclient.GetUserByIdentifierRequest, _ ...grpc.CallOption) (*userclient.UserResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.userResp, nil
}

func (f *fakeUser) ExistsByIdentifier(_ context.Context, _ *userclient.ExistsByIdentifierRequest, _ ...grpc.CallOption) (*userclient.ExistsByIdentifierResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.exists, nil
}

func (f *fakeUser) GetUsersByIDs(_ context.Context, _ *userclient.GetUsersByIDsRequest, _ ...grpc.CallOption) (*userclient.GetUsersByIDsResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.usersResp, nil
}

func TestGetByIDMapsEntityToUser(t *testing.T) {
	fake := &fakeUser{userResp: &userclient.UserResponse{User: &userclient.UserEntity{
		UserId:        "323130844539310080",
		Identifier:    "assistant",
		AccountType:   string(model.AccountTypeAgent),
		AvatarMediaId: "59109426052726784",
		AvatarUrl:     "/media/avatars/59109426052726784",
		CreatedAt:     1781318385000, // 2026-06-13T02:39:45Z 的 UnixMilli
	}}}
	user, err := NewAccountClient(fake).GetByID(context.Background(), "323130844539310080")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if user.AccountID != "323130844539310080" || user.UserID != "323130844539310080" {
		t.Fatalf("account id not mapped: %+v", user)
	}
	if user.AccountType != model.AccountTypeAgent {
		t.Fatalf("account_type = %q, want agent", user.AccountType)
	}
	if user.AvatarMediaID != "59109426052726784" {
		t.Fatalf("avatar_media_id = %q, want decimal string preserved", user.AvatarMediaID)
	}
	if user.CreatedAt.IsZero() {
		t.Fatal("created_at should decode from UnixMilli")
	}
}

func TestListByIDsMapsAll(t *testing.T) {
	fake := &fakeUser{usersResp: &userclient.GetUsersByIDsResponse{Users: []*userclient.UserEntity{
		{UserId: "1", AccountType: string(model.AccountTypeUser)},
		{UserId: "2", AccountType: string(model.AccountTypeAgent)},
	}}}
	users, err := NewAccountClient(fake).ListByIDs(context.Background(), []string{"1", "2"})
	if err != nil {
		t.Fatalf("ListByIDs: %v", err)
	}
	if len(users) != 2 || users[0].AccountID != "1" || users[1].AccountType != model.AccountTypeAgent {
		t.Fatalf("unexpected mapping: %+v", users)
	}
}

func TestExistsByIdentifierMaps(t *testing.T) {
	fake := &fakeUser{exists: &userclient.ExistsByIdentifierResponse{Identifier: "assistant", Exists: true}}
	exists, err := NewAccountClient(fake).ExistsByIdentifier(context.Background(), "assistant")
	if err != nil {
		t.Fatalf("ExistsByIdentifier: %v", err)
	}
	if !exists {
		t.Fatal("expected exists=true")
	}
}

// gRPC status 错误经 rpcerror.FromStatus 还原成 apperror，再 ToStatus 应得回原码（不被吞成 Unknown）。
func TestGetByIDPropagatesNotFoundCode(t *testing.T) {
	fake := &fakeUser{err: status.Error(codes.NotFound, "account not found")}
	_, err := NewAccountClient(fake).GetByID(context.Background(), "999")
	if err == nil {
		t.Fatal("expected error")
	}
	if st, _ := status.FromError(rpcerror.ToStatus(err)); st.Code() != codes.NotFound {
		t.Fatalf("round-trip code = %s, want NotFound", st.Code())
	}
}

// 账号写在 agent-api HTTP 不可达：必须 fail-loud，禁止静默假成功(rule 1/2)。
func TestAccountWritesFailLoud(t *testing.T) {
	c := NewAccountClient(&fakeUser{})
	if _, err := c.Create(context.Background(), model.User{}); err == nil {
		t.Fatal("Create must fail loud")
	}
	if _, err := c.UpdateAvatar(context.Background(), "1", "2", "/u"); err == nil {
		t.Fatal("UpdateAvatar must fail loud")
	}
	if _, err := c.UpdateProfile(context.Background(), "1", repository.AccountProfilePatch{}); err == nil {
		t.Fatal("UpdateProfile must fail loud")
	}
	if _, err := c.RenameIdentifier(context.Background(), "a", "b"); err == nil {
		t.Fatal("RenameIdentifier must fail loud")
	}
	if _, err := c.ListByAccountType(context.Background(), model.AccountTypeUser); err == nil {
		t.Fatal("ListByAccountType must fail loud")
	}
}
