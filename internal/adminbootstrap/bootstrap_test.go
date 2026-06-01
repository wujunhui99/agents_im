package adminbootstrap

import (
	"context"
	"testing"

	authlogic "github.com/wujunhui99/agents_im/internal/auth/logic"
	authrepo "github.com/wujunhui99/agents_im/internal/auth/repository"
	"github.com/wujunhui99/agents_im/pkg/config"
	"github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/common/share/model"
	"github.com/wujunhui99/agents_im/internal/repository"
)

func TestEnsureAdminAccountCreatesLoginableAdmin(t *testing.T) {
	ctx := context.Background()
	accounts := repository.NewMemoryRepository()
	credentials := authrepo.NewMemoryRepository()
	users := logic.NewUserLogic(accounts)

	created, err := EnsureAdminAccount(ctx, Config{
		Identifier:  "amin",
		Password:    "unit-test-admin-password",
		DisplayName: "管理后台管理员",
	}, users, credentials)
	if err != nil {
		t.Fatalf("ensure admin account: %v", err)
	}
	if !created {
		t.Fatal("expected first ensure call to create admin account")
	}

	account, err := accounts.GetByIdentifier(ctx, "amin")
	if err != nil {
		t.Fatalf("get created account: %v", err)
	}
	if account.AccountType != model.AccountTypeAdmin {
		t.Fatalf("account type = %q, want admin", account.AccountType)
	}

	credential, err := credentials.GetByIdentifier(ctx, "amin")
	if err != nil {
		t.Fatalf("get created credential: %v", err)
	}
	if credential.UserID != account.UserID {
		t.Fatalf("credential user_id = %q, want %q", credential.UserID, account.UserID)
	}
	if !authlogic.NewPasswordHasher().Verify("unit-test-admin-password", credential.Salt, credential.PasswordHash, credential.HashVersion) {
		t.Fatal("created credential does not verify requested password")
	}
}

func TestEnsureAdminAccountIsIdempotent(t *testing.T) {
	ctx := context.Background()
	accounts := repository.NewMemoryRepository()
	credentials := authrepo.NewMemoryRepository()
	users := logic.NewUserLogic(accounts)
	cfg := Config{Identifier: "amin", Password: "unit-test-admin-password"}

	created, err := EnsureAdminAccount(ctx, cfg, users, credentials)
	if err != nil || !created {
		t.Fatalf("first ensure created=%v err=%v", created, err)
	}
	created, err = EnsureAdminAccount(ctx, cfg, users, credentials)
	if err != nil {
		t.Fatalf("second ensure: %v", err)
	}
	if created {
		t.Fatal("second ensure should not recreate an existing admin account")
	}
}

func TestEnsureAdminAccountDisabledWithoutPassword(t *testing.T) {
	created, err := EnsureAdminAccount(context.Background(), Config{Identifier: "amin"}, logic.NewUserLogic(repository.NewMemoryRepository()), authrepo.NewMemoryRepository())
	if err != nil {
		t.Fatalf("ensure disabled admin account: %v", err)
	}
	if created {
		t.Fatal("admin bootstrap should be disabled when password is empty")
	}
}

func TestConfigFromAPIConfigUsesConfiguredAdminBootstrap(t *testing.T) {
	cfg := FromAPIConfig(config.APIConfig{
		AdminBootstrap: config.AdminBootstrapConfig{
			Identifier:  "amin",
			Password:    "unit-test-admin-password",
			DisplayName: "管理后台管理员",
		},
	})
	if cfg.Identifier != "amin" || cfg.Password != "unit-test-admin-password" || cfg.DisplayName != "管理后台管理员" {
		t.Fatalf("unexpected bootstrap config: %+v", cfg)
	}
}
