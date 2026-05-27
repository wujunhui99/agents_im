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
