package tests

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/wujunhui99/agents_im/internal/apperror"
	authlogic "github.com/wujunhui99/agents_im/internal/auth/logic"
	authrepo "github.com/wujunhui99/agents_im/internal/auth/repository"
	"github.com/wujunhui99/agents_im/internal/auth/token"
	"github.com/wujunhui99/agents_im/internal/auth/useradapter"
	"github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/model"
	"github.com/wujunhui99/agents_im/internal/repository"
	authsvc "github.com/wujunhui99/agents_im/internal/servicecontext/auth"
	userpb "github.com/wujunhui99/agents_im/service/user/rpc/user"
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
	assertNumericSnowflakeID(t, userAccount.UserID)

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
	assertNumericSnowflakeID(t, agent.UserID)

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

func TestAccountLogicCreateAccountUsesNumericSnowflakeID(t *testing.T) {
	accountLogic := logic.NewAccountLogic(repository.NewMemoryRepository())
	ctx := context.Background()

	account, err := accountLogic.CreateAccount(ctx, logic.CreateAccountRequest{
		Identifier:  "account_numeric_001",
		DisplayName: "Numeric Account",
	})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}

	assertNumericSnowflakeID(t, account.UserID)
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
	assertNumericSnowflakeID(t, defaultUser.UserID)

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
	assertNumericSnowflakeID(t, explicitAgent.UserID)

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
	credentialRepo := authrepo.NewMemoryRepository()
	serviceContext := authsvc.NewServiceContextWithOptions(
		credentialRepo,
		useradapter.NewLogicClient(userLogic),
		token.NewHMACTokenManager("test-secret", 0),
		testAuthOptions(credentialRepo),
	)
	mux := newAuthGoZeroRouter(t, serviceContext)

	codeResp := performJSON(mux, "POST", "/auth/register/email-code", `{"email":"auth_agent_attempt@example.com"}`)
	if codeResp.Code != 200 {
		t.Fatalf("email code status = %d, body = %s", codeResp.Code, codeResp.Body.String())
	}

	registerResp := performJSON(mux, "POST", "/auth/register", `{"identifier":"auth_agent_attempt","email":"auth_agent_attempt@example.com","email_verification_code":"`+testRegistrationCode+`","password":"correct-password","account_type":"admin"}`)
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

	user := &userpb.UserEntity{AccountType: string(model.AccountTypeAdmin)}
	if user.GetAccountType() != string(model.AccountTypeAdmin) {
		t.Fatalf("rpc user account_type = %q, want %q", user.GetAccountType(), model.AccountTypeAdmin)
	}
}

func assertNumericSnowflakeID(t *testing.T, id string) {
	t.Helper()

	if strings.HasPrefix(id, "usr_") || strings.HasPrefix(id, "agt_") || strings.HasPrefix(id, "grp_") {
		t.Fatalf("id %q must not use legacy prefixes", id)
	}
	if len(id) < 15 || len(id) > 20 {
		t.Fatalf("id %q length = %d, want Snowflake numeric string length 15..20", id, len(id))
	}
	parsed, err := strconv.ParseUint(id, 10, 64)
	if err != nil {
		t.Fatalf("id %q is not a numeric Snowflake string: %v", id, err)
	}
	if parsed == 0 {
		t.Fatalf("id %q must be non-zero", id)
	}
}
