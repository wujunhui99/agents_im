package bootstrap

import (
	"context"
	"errors"
	"testing"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	commonconfig "github.com/wujunhui99/agents_im/pkg/config"
	"github.com/wujunhui99/agents_im/pkg/rpcerror"
	authpb "github.com/wujunhui99/agents_im/service/auth/rpc/auth"
	userpb "github.com/wujunhui99/agents_im/service/user/rpc/user"

	"google.golang.org/grpc"
)

type fakeBootstrapUserRPC struct {
	users     map[string]*userpb.UserEntity
	createReq *userpb.CreateUserRequest
}

func (f *fakeBootstrapUserRPC) CreateUser(_ context.Context, in *userpb.CreateUserRequest, _ ...grpc.CallOption) (*userpb.UserResponse, error) {
	f.createReq = in
	user := &userpb.UserEntity{
		UserId:      "admin-1",
		Identifier:  in.GetIdentifier(),
		DisplayName: in.GetDisplayName(),
		Name:        in.GetName(),
		AccountType: in.GetAccountType(),
	}
	if f.users == nil {
		f.users = map[string]*userpb.UserEntity{}
	}
	f.users[user.GetIdentifier()] = user
	return &userpb.UserResponse{User: user}, nil
}

func (f *fakeBootstrapUserRPC) GetUserByIdentifier(_ context.Context, in *userpb.GetUserByIdentifierRequest, _ ...grpc.CallOption) (*userpb.UserResponse, error) {
	if user, ok := f.users[in.GetIdentifier()]; ok {
		return &userpb.UserResponse{User: user}, nil
	}
	return nil, rpcerror.ToStatus(apperror.NotFound("account not found"))
}

func (f *fakeBootstrapUserRPC) ExistsByIdentifier(context.Context, *userpb.ExistsByIdentifierRequest, ...grpc.CallOption) (*userpb.ExistsByIdentifierResponse, error) {
	panic("fakeBootstrapUserRPC.ExistsByIdentifier: unused")
}

func (f *fakeBootstrapUserRPC) GetUserByID(context.Context, *userpb.GetUserByIDRequest, ...grpc.CallOption) (*userpb.UserResponse, error) {
	panic("fakeBootstrapUserRPC.GetUserByID: unused")
}

func (f *fakeBootstrapUserRPC) GetUsersByIDs(context.Context, *userpb.GetUsersByIDsRequest, ...grpc.CallOption) (*userpb.GetUsersByIDsResponse, error) {
	panic("fakeBootstrapUserRPC.GetUsersByIDs: unused")
}

func (f *fakeBootstrapUserRPC) SearchAccounts(context.Context, *userpb.SearchAccountsRequest, ...grpc.CallOption) (*userpb.SearchAccountsResponse, error) {
	panic("fakeBootstrapUserRPC.SearchAccounts: unused")
}

func (f *fakeBootstrapUserRPC) CountAccounts(context.Context, *userpb.CountAccountsRequest, ...grpc.CallOption) (*userpb.CountAccountsResponse, error) {
	panic("fakeBootstrapUserRPC.CountAccounts: unused")
}

func (f *fakeBootstrapUserRPC) UpdateUserProfile(context.Context, *userpb.UpdateUserProfileRequest, ...grpc.CallOption) (*userpb.UserResponse, error) {
	panic("fakeBootstrapUserRPC.UpdateUserProfile: unused")
}

func (f *fakeBootstrapUserRPC) UpdateUserAvatar(context.Context, *userpb.UpdateUserAvatarRequest, ...grpc.CallOption) (*userpb.UserResponse, error) {
	panic("fakeBootstrapUserRPC.UpdateUserAvatar: unused")
}

func (f *fakeBootstrapUserRPC) CreateTestAccount(context.Context, *userpb.CreateTestAccountRequest, ...grpc.CallOption) (*userpb.CreateTestAccountResponse, error) {
	panic("fakeBootstrapUserRPC.CreateTestAccount: unused")
}

type fakeBootstrapAuthRPC struct {
	ensureAdminReq     *authpb.EnsureAdminCredentialRequest
	ensureAdminCreated bool
	ensureAdminErr     error
}

func (f *fakeBootstrapAuthRPC) EnsureAdminCredential(_ context.Context, in *authpb.EnsureAdminCredentialRequest, _ ...grpc.CallOption) (*authpb.EnsureAdminCredentialResponse, error) {
	f.ensureAdminReq = in
	if f.ensureAdminErr != nil {
		return nil, f.ensureAdminErr
	}
	return &authpb.EnsureAdminCredentialResponse{
		UserId:     in.GetUserId(),
		Identifier: in.GetIdentifier(),
		Created:    f.ensureAdminCreated,
	}, nil
}

func (f *fakeBootstrapAuthRPC) RequestRegistrationEmailCode(context.Context, *authpb.RegistrationEmailCodeRequest, ...grpc.CallOption) (*authpb.RegistrationEmailCodeResponse, error) {
	panic("fakeBootstrapAuthRPC.RequestRegistrationEmailCode: unused")
}

func (f *fakeBootstrapAuthRPC) Register(context.Context, *authpb.RegisterRequest, ...grpc.CallOption) (*authpb.AuthResponse, error) {
	panic("fakeBootstrapAuthRPC.Register: unused")
}

func (f *fakeBootstrapAuthRPC) Login(context.Context, *authpb.LoginRequest, ...grpc.CallOption) (*authpb.AuthResponse, error) {
	panic("fakeBootstrapAuthRPC.Login: unused")
}

func (f *fakeBootstrapAuthRPC) ValidateToken(context.Context, *authpb.ValidateTokenRequest, ...grpc.CallOption) (*authpb.ValidateTokenResponse, error) {
	panic("fakeBootstrapAuthRPC.ValidateToken: unused")
}

func (f *fakeBootstrapAuthRPC) ParseToken(context.Context, *authpb.ValidateTokenRequest, ...grpc.CallOption) (*authpb.ValidateTokenResponse, error) {
	panic("fakeBootstrapAuthRPC.ParseToken: unused")
}

func (f *fakeBootstrapAuthRPC) EnsureTestCredential(context.Context, *authpb.EnsureTestCredentialRequest, ...grpc.CallOption) (*authpb.EnsureTestCredentialResponse, error) {
	panic("fakeBootstrapAuthRPC.EnsureTestCredential: unused")
}

func TestEnsureAdminAccountCreatesAccountAndCredential(t *testing.T) {
	users := &fakeBootstrapUserRPC{}
	auth := &fakeBootstrapAuthRPC{ensureAdminCreated: true}

	created, err := EnsureAdminAccount(context.Background(), commonconfig.AdminBootstrapConfig{
		Identifier:  "amin",
		Password:    "admin-secret-pw",
		DisplayName: "管理后台管理员",
	}, users, auth)
	if err != nil {
		t.Fatalf("EnsureAdminAccount: %v", err)
	}
	if !created {
		t.Fatal("first ensure should report created work")
	}
	if users.createReq.GetAccountType() != "admin" {
		t.Fatalf("created account_type = %q, want admin", users.createReq.GetAccountType())
	}
	if auth.ensureAdminReq.GetUserId() != "admin-1" || auth.ensureAdminReq.GetPassword() != "admin-secret-pw" {
		t.Fatalf("unexpected auth ensure request: %+v", auth.ensureAdminReq)
	}
}

func TestEnsureAdminAccountUsesExistingAdmin(t *testing.T) {
	users := &fakeBootstrapUserRPC{users: map[string]*userpb.UserEntity{
		"amin": {UserId: "admin-1", Identifier: "amin", AccountType: "admin"},
	}}
	auth := &fakeBootstrapAuthRPC{}

	created, err := EnsureAdminAccount(context.Background(), commonconfig.AdminBootstrapConfig{
		Identifier: "amin",
		Password:   "admin-secret-pw",
	}, users, auth)
	if err != nil {
		t.Fatalf("EnsureAdminAccount: %v", err)
	}
	if created {
		t.Fatal("existing admin with existing credential should not report created work")
	}
	if users.createReq != nil {
		t.Fatal("existing admin account should not be recreated")
	}
	if auth.ensureAdminReq.GetUserId() != "admin-1" {
		t.Fatalf("auth ensure user_id = %q, want admin-1", auth.ensureAdminReq.GetUserId())
	}
}

func TestEnsureAdminAccountDisabledWithoutPassword(t *testing.T) {
	users := &fakeBootstrapUserRPC{}
	auth := &fakeBootstrapAuthRPC{}

	created, err := EnsureAdminAccount(context.Background(), commonconfig.AdminBootstrapConfig{Identifier: "amin"}, users, auth)
	if err != nil {
		t.Fatalf("EnsureAdminAccount: %v", err)
	}
	if created {
		t.Fatal("bootstrap should be disabled when password is empty")
	}
	if users.createReq != nil || auth.ensureAdminReq != nil {
		t.Fatal("disabled bootstrap should not call downstream RPCs")
	}
}

func TestEnsureAdminAccountRejectsExistingNonAdmin(t *testing.T) {
	users := &fakeBootstrapUserRPC{users: map[string]*userpb.UserEntity{
		"amin": {UserId: "user-1", Identifier: "amin", AccountType: "user"},
	}}
	auth := &fakeBootstrapAuthRPC{}

	if _, err := EnsureAdminAccount(context.Background(), commonconfig.AdminBootstrapConfig{
		Identifier: "amin",
		Password:   "admin-secret-pw",
	}, users, auth); err == nil {
		t.Fatal("existing non-admin account should fail")
	}
	if auth.ensureAdminReq != nil {
		t.Fatal("non-admin account should not get credential ensure")
	}
}

func TestEnsureAdminAccountPropagatesAuthError(t *testing.T) {
	users := &fakeBootstrapUserRPC{users: map[string]*userpb.UserEntity{
		"amin": {UserId: "admin-1", Identifier: "amin", AccountType: "admin"},
	}}
	auth := &fakeBootstrapAuthRPC{ensureAdminErr: errors.New("auth unavailable")}

	if _, err := EnsureAdminAccount(context.Background(), commonconfig.AdminBootstrapConfig{
		Identifier: "amin",
		Password:   "admin-secret-pw",
	}, users, auth); err == nil {
		t.Fatal("auth error should propagate")
	}
}
