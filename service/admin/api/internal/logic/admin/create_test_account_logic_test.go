package admin

import (
	"context"
	"errors"
	"testing"

	"github.com/wujunhui99/agents_im/service/admin/api/internal/svc"
	"github.com/wujunhui99/agents_im/service/admin/api/internal/types"
	authpb "github.com/wujunhui99/agents_im/service/auth/rpc/auth"
	userpb "github.com/wujunhui99/agents_im/service/user/rpc/user"

	"google.golang.org/grpc"
)

// --- fakes ---

type fakeUserRPC struct {
	createReq  *userpb.CreateTestAccountRequest
	createResp *userpb.CreateTestAccountResponse
	createErr  error
}

func (f *fakeUserRPC) CreateTestAccount(_ context.Context, in *userpb.CreateTestAccountRequest, _ ...grpc.CallOption) (*userpb.CreateTestAccountResponse, error) {
	f.createReq = in
	return f.createResp, f.createErr
}

func (f *fakeUserRPC) CreateUser(context.Context, *userpb.CreateUserRequest, ...grpc.CallOption) (*userpb.UserResponse, error) {
	panic("fakeUserRPC.CreateUser: unused")
}

func (f *fakeUserRPC) GetUserByIdentifier(context.Context, *userpb.GetUserByIdentifierRequest, ...grpc.CallOption) (*userpb.UserResponse, error) {
	panic("fakeUserRPC.GetUserByIdentifier: unused")
}

func (f *fakeUserRPC) ExistsByIdentifier(context.Context, *userpb.ExistsByIdentifierRequest, ...grpc.CallOption) (*userpb.ExistsByIdentifierResponse, error) {
	panic("fakeUserRPC.ExistsByIdentifier: unused")
}

func (f *fakeUserRPC) GetUserByID(context.Context, *userpb.GetUserByIDRequest, ...grpc.CallOption) (*userpb.UserResponse, error) {
	panic("fakeUserRPC.GetUserByID: unused")
}

func (f *fakeUserRPC) GetUsersByIDs(context.Context, *userpb.GetUsersByIDsRequest, ...grpc.CallOption) (*userpb.GetUsersByIDsResponse, error) {
	panic("fakeUserRPC.GetUsersByIDs: unused")
}

func (f *fakeUserRPC) UpdateUserProfile(context.Context, *userpb.UpdateUserProfileRequest, ...grpc.CallOption) (*userpb.UserResponse, error) {
	panic("fakeUserRPC.UpdateUserProfile: unused")
}

func (f *fakeUserRPC) UpdateUserAvatar(context.Context, *userpb.UpdateUserAvatarRequest, ...grpc.CallOption) (*userpb.UserResponse, error) {
	panic("fakeUserRPC.UpdateUserAvatar: unused")
}

type fakeAuthRPC struct {
	ensureReq *authpb.EnsureTestCredentialRequest
	ensureErr error
}

func (f *fakeAuthRPC) EnsureTestCredential(_ context.Context, in *authpb.EnsureTestCredentialRequest, _ ...grpc.CallOption) (*authpb.EnsureTestCredentialResponse, error) {
	f.ensureReq = in
	if f.ensureErr != nil {
		return nil, f.ensureErr
	}
	return &authpb.EnsureTestCredentialResponse{UserId: in.GetUserId(), Identifier: in.GetIdentifier()}, nil
}

func (f *fakeAuthRPC) EnsureAdminCredential(context.Context, *authpb.EnsureAdminCredentialRequest, ...grpc.CallOption) (*authpb.EnsureAdminCredentialResponse, error) {
	panic("fakeAuthRPC.EnsureAdminCredential: unused")
}

func (f *fakeAuthRPC) RequestRegistrationEmailCode(context.Context, *authpb.RegistrationEmailCodeRequest, ...grpc.CallOption) (*authpb.RegistrationEmailCodeResponse, error) {
	panic("fakeAuthRPC.RequestRegistrationEmailCode: unused")
}

func (f *fakeAuthRPC) Register(context.Context, *authpb.RegisterRequest, ...grpc.CallOption) (*authpb.AuthResponse, error) {
	panic("fakeAuthRPC.Register: unused")
}

func (f *fakeAuthRPC) Login(context.Context, *authpb.LoginRequest, ...grpc.CallOption) (*authpb.AuthResponse, error) {
	panic("fakeAuthRPC.Login: unused")
}

func (f *fakeAuthRPC) ValidateToken(context.Context, *authpb.ValidateTokenRequest, ...grpc.CallOption) (*authpb.ValidateTokenResponse, error) {
	panic("fakeAuthRPC.ValidateToken: unused")
}

func (f *fakeAuthRPC) ParseToken(context.Context, *authpb.ValidateTokenRequest, ...grpc.CallOption) (*authpb.ValidateTokenResponse, error) {
	panic("fakeAuthRPC.ParseToken: unused")
}

func testUserEntity() *userpb.UserEntity {
	return &userpb.UserEntity{
		UserId:      "acct-9",
		Identifier:  "tester",
		DisplayName: "测试账户",
		Name:        "测试账户",
		AccountType: "test",
	}
}

// --- tests ---

func TestCreateTestAccountGeneratesPasswordAndOrchestratesRPCs(t *testing.T) {
	userRPC := &fakeUserRPC{createResp: &userpb.CreateTestAccountResponse{User: testUserEntity()}}
	authRPC := &fakeAuthRPC{}
	logic := NewCreateTestAccountLogic(context.Background(), &svc.ServiceContext{UserRPC: userRPC, AuthRPC: authRPC})

	resp, err := logic.CreateTestAccount(&types.AdminTestAccountCreateReq{Identifier: "tester"})
	if err != nil {
		t.Fatalf("CreateTestAccount: %v", err)
	}
	if userRPC.createReq.GetIdentifier() != "tester" {
		t.Fatalf("user-rpc identifier = %q", userRPC.createReq.GetIdentifier())
	}
	if len(resp.Data.Password) != generatedPasswordLength {
		t.Fatalf("generated password length = %d, want %d", len(resp.Data.Password), generatedPasswordLength)
	}
	if authRPC.ensureReq.GetPassword() != resp.Data.Password {
		t.Fatal("auth-rpc should receive the same password returned to the operator")
	}
	if authRPC.ensureReq.GetUserId() != "acct-9" {
		t.Fatalf("auth-rpc user_id = %q, want acct-9", authRPC.ensureReq.GetUserId())
	}
	if resp.Data.User.AccountType != "test" {
		t.Fatalf("account_type = %q, want test", resp.Data.User.AccountType)
	}
}

func TestCreateTestAccountUsesProvidedPasswordAndReportsExisting(t *testing.T) {
	userRPC := &fakeUserRPC{createResp: &userpb.CreateTestAccountResponse{User: testUserEntity(), AlreadyExists: true}}
	authRPC := &fakeAuthRPC{}
	logic := NewCreateTestAccountLogic(context.Background(), &svc.ServiceContext{UserRPC: userRPC, AuthRPC: authRPC})

	resp, err := logic.CreateTestAccount(&types.AdminTestAccountCreateReq{Identifier: "tester", Password: "chosen-password"})
	if err != nil {
		t.Fatalf("CreateTestAccount: %v", err)
	}
	if resp.Data.Password != "chosen-password" || authRPC.ensureReq.GetPassword() != "chosen-password" {
		t.Fatal("provided password should be used as-is")
	}
	if !resp.Data.AlreadyExisted {
		t.Fatal("alreadyExisted should surface user-rpc already_exists")
	}
}

func TestCreateTestAccountPropagatesRPCErrors(t *testing.T) {
	logic := NewCreateTestAccountLogic(context.Background(), &svc.ServiceContext{
		UserRPC: &fakeUserRPC{createErr: errors.New("user rpc down")},
		AuthRPC: &fakeAuthRPC{},
	})
	if _, err := logic.CreateTestAccount(&types.AdminTestAccountCreateReq{Identifier: "tester"}); err == nil {
		t.Fatal("user-rpc error should propagate")
	}

	logic = NewCreateTestAccountLogic(context.Background(), &svc.ServiceContext{
		UserRPC: &fakeUserRPC{createResp: &userpb.CreateTestAccountResponse{User: testUserEntity()}},
		AuthRPC: &fakeAuthRPC{ensureErr: errors.New("auth rpc down")},
	})
	if _, err := logic.CreateTestAccount(&types.AdminTestAccountCreateReq{Identifier: "tester"}); err == nil {
		t.Fatal("auth-rpc error should propagate")
	}

	if _, err := logic.CreateTestAccount(&types.AdminTestAccountCreateReq{Identifier: "   "}); err == nil {
		t.Fatal("blank identifier should be rejected")
	}
}
