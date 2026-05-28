package config_test

import (
	"os"
	"path/filepath"
	"testing"

	commonconfig "github.com/wujunhui99/agents_im/internal/config"
	authconfig "github.com/wujunhui99/agents_im/service/auth/rpc/internal/config"
	"github.com/zeromicro/go-zero/core/conf"
)

func TestAuthRPCConfigLoadsRuntimeDependencies(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "auth-rpc.yaml")
	content := []byte(`Name: auth-rpc
ListenOn: 127.0.0.1:19091
TokenAuth:
  AccessSecret: "[REDACTED]"
  AccessExpire: 3600
AdminBootstrap:
  Identifier: amin
  Password: "[REDACTED]"
  DisplayName: 管理后台管理员
StorageDriver: postgres
DataSource: "[REDACTED]"
MailRPC:
  Endpoints:
    - 127.0.0.1:9095
  Timeout: 5000
`)
	if err := os.WriteFile(configPath, content, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	var cfg authconfig.Config
	if err := conf.Load(configPath, &cfg); err != nil {
		t.Fatalf("load auth rpc config: %v", err)
	}
	if cfg.TokenAuth != (commonconfig.JWTAuthConfig{AccessSecret: "[REDACTED]", AccessExpire: 3600}) {
		t.Fatalf("token auth config mismatch: accessSecretMatches=%v accessExpire=%d", cfg.TokenAuth.AccessSecret == "[REDACTED]", cfg.TokenAuth.AccessExpire)
	}
	if cfg.AdminBootstrap.Identifier != "amin" || cfg.AdminBootstrap.Password != "[REDACTED]" {
		t.Fatalf("admin bootstrap config mismatch: identifier=%q passwordMatches=%v", cfg.AdminBootstrap.Identifier, cfg.AdminBootstrap.Password == "[REDACTED]")
	}
	if cfg.StorageDriver != commonconfig.StorageDriverPostgres || cfg.DataSource == "" {
		t.Fatalf("storage config mismatch: driver=%q dataSourceEmpty=%v", cfg.StorageDriver, cfg.DataSource == "")
	}
	if len(cfg.MailRPC.Endpoints) != 1 || cfg.MailRPC.Endpoints[0] != "127.0.0.1:9095" || cfg.MailRPC.Timeout != 5000 {
		t.Fatalf("mail rpc config mismatch: %+v", cfg.MailRPC)
	}
}

func TestAuthRPCConfigResolvesEnvPlaceholdersForAdminBootstrap(t *testing.T) {
	t.Setenv("ADMIN_BOOTSTRAP_IDENTIFIER", "admin")
	t.Setenv("ADMIN_BOOTSTRAP_PASSWORD", "unit-test-admin-password")
	t.Setenv("ADMIN_BOOTSTRAP_DISPLAY_NAME", "管理后台管理员")

	configPath := filepath.Join(t.TempDir(), "auth-rpc.yaml")
	if err := os.WriteFile(configPath, []byte(`Name: auth-rpc
ListenOn: 127.0.0.1:0
TokenAuth:
  AccessSecret: test-secret
  AccessExpire: 86400
AdminBootstrap:
  Identifier: ${ADMIN_BOOTSTRAP_IDENTIFIER}
  Password: ${ADMIN_BOOTSTRAP_PASSWORD}
  DisplayName: ${ADMIN_BOOTSTRAP_DISPLAY_NAME}
StorageDriver: memory
`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	var cfg authconfig.Config
	conf.MustLoad(configPath, &cfg)
	cfg.ResolveEnvPlaceholders()

	if cfg.AdminBootstrap.Identifier != "admin" {
		t.Fatalf("identifier = %q", cfg.AdminBootstrap.Identifier)
	}
	if cfg.AdminBootstrap.Password != "unit-test-admin-password" {
		t.Fatalf("password placeholder was not resolved")
	}
	if cfg.AdminBootstrap.DisplayName != "管理后台管理员" {
		t.Fatalf("display name = %q", cfg.AdminBootstrap.DisplayName)
	}
}
