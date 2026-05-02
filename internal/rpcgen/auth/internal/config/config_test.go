package config_test

import (
	"os"
	"path/filepath"
	"testing"

	commonconfig "github.com/wujunhui99/agents_im/internal/config"
	authconfig "github.com/wujunhui99/agents_im/internal/rpcgen/auth/internal/config"
	"github.com/zeromicro/go-zero/core/conf"
)

func TestAuthRPCConfigLoadsTokenAuthWithoutGoZeroAuthConflict(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "auth-rpc.yaml")
	content := []byte(`Name: auth-rpc
ListenOn: 127.0.0.1:19091
TokenAuth:
  AccessSecret: test-auth-rpc-secret
  AccessExpire: 3600
StorageDriver: postgres
DataSource: postgres://example.invalid/agents_im
`)
	if err := os.WriteFile(configPath, content, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	var cfg authconfig.Config
	if err := conf.Load(configPath, &cfg); err != nil {
		t.Fatalf("load auth rpc config: %v", err)
	}
	if cfg.TokenAuth != (commonconfig.JWTAuthConfig{AccessSecret: "test-auth-rpc-secret", AccessExpire: 3600}) {
		t.Fatalf("token auth config mismatch: %+v", cfg.TokenAuth)
	}
	if cfg.StorageDriver != commonconfig.StorageDriverPostgres || cfg.DataSource == "" {
		t.Fatalf("storage config mismatch: driver=%q dataSourceEmpty=%v", cfg.StorageDriver, cfg.DataSource == "")
	}
}
