package config_test

import (
	"os"
	"path/filepath"
	"testing"

	mailconfig "github.com/wujunhui99/agents_im/internal/rpcgen/mail/internal/config"
	"github.com/zeromicro/go-zero/core/conf"
)

func TestMailRPCConfigLoadsTencentSESSettings(t *testing.T) {
	t.Setenv("TENCENT_SES_SECRET_ID", "test-secret-id")
	t.Setenv("TENCENT_SES_SECRET_KEY", "test-secret-key")
	t.Setenv("TENCENT_SES_REGION", "ap-hongkong")
	t.Setenv("TENCENT_SES_FROM_EMAIL", "noreply@agenticim.xyz")
	t.Setenv("TENCENT_SES_DEFAULT_TEMPLATE_ID", "177952")

	configPath := filepath.Join(t.TempDir(), "mail-rpc.yaml")
	content := []byte(`Name: mail-rpc
ListenOn: 127.0.0.1:19095
TencentSES:
  SecretID: ${TENCENT_SES_SECRET_ID}
  SecretKey: ${TENCENT_SES_SECRET_KEY}
  Region: ${TENCENT_SES_REGION}
  Endpoint: https://ses.tencentcloudapi.com
  FromEmailAddress: ${TENCENT_SES_FROM_EMAIL}
  DefaultTemplateID: ${TENCENT_SES_DEFAULT_TEMPLATE_ID}
`)
	if err := os.WriteFile(configPath, content, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	var cfg mailconfig.Config
	if err := conf.Load(configPath, &cfg); err != nil {
		t.Fatalf("load mail rpc config: %v", err)
	}
	resolved := cfg.TencentSES.WithDefaults()
	if resolved.Region != "ap-hongkong" || resolved.DefaultTemplateID != "177952" {
		t.Fatalf("mail rpc config mismatch: %+v", resolved)
	}
	if err := resolved.Validate(); err != nil {
		t.Fatalf("expected loaded config to validate: %v", err)
	}
}
