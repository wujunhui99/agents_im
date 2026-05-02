package tests

import (
	"context"
	"strings"
	"testing"

	"github.com/wujunhui99/agents_im/internal/apperror"
	authlogic "github.com/wujunhui99/agents_im/internal/auth/logic"
	authrepo "github.com/wujunhui99/agents_im/internal/auth/repository"
	authsvc "github.com/wujunhui99/agents_im/internal/auth/svc"
	"github.com/wujunhui99/agents_im/internal/auth/token"
	"github.com/wujunhui99/agents_im/internal/auth/useradapter"
	"github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/model"
	"github.com/wujunhui99/agents_im/internal/repository"
	"github.com/wujunhui99/agents_im/proto/userpb"
)

func TestAccountTypeDefaultsAndExplicitInternalCreate(t *testing.T) {
	userLogic := logic.NewUserLogic(repository.NewMemoryRepository())
	ctx := context.Background()

	userAccount, err := userLogic.CreateUser(ctx, logic.CreateUserRequest{Identifier: "user_001"})
	if err != nil {
		t.Fatalf("create default user: %v", err)
	}
	if userAccount.AccountType != string(model.AccountTypeUser) {
		t.Fatalf("default account_type = %q, want %q", userAccount.AccountType, model.AccountTypeUser)
	}

	_, err = userLogic.CreateUser(ctx, logic.CreateUserRequest{
		Identifier:  "legacy_normal_001",
		AccountType: "normal",
	})
	if err == nil || apperror.From(err).Code != apperror.CodeInvalidArgument {
		t.Fatalf("legacy normal account_type error = %v, want INVALID_ARGUMENT", err)
	}

	agent, err := userLogic.CreateUser(ctx, logic.CreateUserRequest{
		Identifier:  "agent_001",
		AccountType: string(model.AccountTypeAgent),
	})
	if err != nil {
		t.Fatalf("create agent user: %v", err)
	}
	if agent.AccountType != string(model.AccountTypeAgent) {
		t.Fatalf("agent account_type = %q, want %q", agent.AccountType, model.AccountTypeAgent)
	}

	admin, err := userLogic.CreateUser(ctx, logic.CreateUserRequest{
		Identifier:  "admin_001",
		AccountType: string(model.AccountTypeAdmin),
	})
	if err != nil {
		t.Fatalf("create admin user: %v", err)
	}
	if admin.AccountType != string(model.AccountTypeAdmin) {
		t.Fatalf("admin account_type = %q, want %q", admin.AccountType, model.AccountTypeAdmin)
	}

	_, err = userLogic.CreateUser(ctx, logic.CreateUserRequest{
		Identifier:  "invalid_001",
		AccountType: "superuser",
	})
	if err == nil {
		t.Fatal("expected invalid account_type to fail")
	}
	appErr := apperror.From(err)
	if appErr.Code != apperror.CodeInvalidArgument || !strings.Contains(appErr.Message, "account_type") {
		t.Fatalf("invalid account_type error = %v, want INVALID_ARGUMENT mentioning account_type", err)
	}
}

func TestMemoryRepositoryAccountTypeSemantics(t *testing.T) {
	repo := repository.NewMemoryRepository()
	ctx := context.Background()

	defaultUser, err := repo.Create(ctx, model.User{
		Identifier:  "repo_user",
		DisplayName: "Repo User",
		Name:        "Repo User",
		Gender:      "unknown",
	})
	if err != nil {
		t.Fatalf("create default repository user: %v", err)
	}
	if defaultUser.AccountType != model.AccountTypeUser {
		t.Fatalf("repository default account_type = %q, want %q", defaultUser.AccountType, model.AccountTypeUser)
	}

	explicitAgent, err := repo.Create(ctx, model.User{
		Identifier:  "repo_agent",
		DisplayName: "Repo Agent",
		Name:        "Repo Agent",
		Gender:      "unknown",
		AccountType: model.AccountTypeAgent,
	})
	if err != nil {
		t.Fatalf("create explicit repository agent: %v", err)
	}
	if explicitAgent.AccountType != model.AccountTypeAgent {
		t.Fatalf("repository explicit account_type = %q, want %q", explicitAgent.AccountType, model.AccountTypeAgent)
	}

	_, err = repo.Create(ctx, model.User{
		Identifier:  "repo_invalid",
		DisplayName: "Repo Invalid",
		Name:        "Repo Invalid",
		Gender:      "unknown",
		AccountType: model.AccountType("root"),
	})
	if err == nil || apperror.From(err).Code != apperror.CodeInvalidArgument {
		t.Fatalf("repository invalid account_type error = %v, want INVALID_ARGUMENT", err)
	}
}

func TestPublicAccountCreateIgnoresAccountTypeAndDefaultsUser(t *testing.T) {
	serviceContext := newTestUserServiceContext()
	mux := newUserGoZeroRouter(t, serviceContext)

	createResp := performJSON(mux, "POST", "/users", `{"identifier":"public_agent_attempt","account_type":"admin"}`)
	if createResp.Code != 200 {
		t.Fatalf("create status = %d, body = %s", createResp.Code, createResp.Body.String())
	}

	var created envelope[logic.UserProfile]
	decodeEnvelope(t, createResp.Body.Bytes(), &created)
	if created.Data.AccountType != string(model.AccountTypeUser) {
		t.Fatalf("public create account_type = %q, want default %q", created.Data.AccountType, model.AccountTypeUser)
	}
}

func TestAuthRegisterCreatesUserAccountTypeByDefault(t *testing.T) {
	userLogic := logic.NewUserLogic(repository.NewMemoryRepository())
	serviceContext := authsvc.NewServiceContext(
		authrepo.NewMemoryRepository(),
		useradapter.NewLogicClient(userLogic),
		token.NewHMACTokenManager("test-secret", 0),
	)
	mux := newAuthGoZeroRouter(t, serviceContext)

	registerResp := performJSON(mux, "POST", "/auth/register", `{"identifier":"auth_agent_attempt","password":"correct-password","account_type":"admin"}`)
	if registerResp.Code != 200 {
		t.Fatalf("register status = %d, body = %s", registerResp.Code, registerResp.Body.String())
	}

	var registered envelope[authlogic.AuthResponse]
	decodeEnvelope(t, registerResp.Body.Bytes(), &registered)

	profile, err := userLogic.GetUserByID(context.Background(), logic.GetUserByIDRequest{UserID: registered.Data.UserID})
	if err != nil {
		t.Fatalf("load registered user: %v", err)
	}
	if profile.AccountType != string(model.AccountTypeUser) {
		t.Fatalf("auth register account_type = %q, want %q", profile.AccountType, model.AccountTypeUser)
	}
}

func TestUserRPCAccountTypeContract(t *testing.T) {
	req := &userpb.CreateUserRequest{AccountType: string(model.AccountTypeAgent)}
	if req.GetAccountType() != string(model.AccountTypeAgent) {
		t.Fatalf("rpc create account_type = %q, want %q", req.GetAccountType(), model.AccountTypeAgent)
	}

	user := &userpb.User{AccountType: string(model.AccountTypeAdmin)}
	if user.GetAccountType() != string(model.AccountTypeAdmin) {
		t.Fatalf("rpc user account_type = %q, want %q", user.GetAccountType(), model.AccountTypeAdmin)
	}
}
