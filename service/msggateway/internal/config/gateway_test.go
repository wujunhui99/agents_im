package config

import (
	"reflect"
	"strings"
	"testing"

	"github.com/zeromicro/go-zero/core/conf"
)

// TestGatewayWSConfigTagDefaults 走 go-zero conf 加载路径（生产 conf.MustLoad 同款），
// 验证 default= struct tag 在 yaml 缺省时填出生产安全默认值。
func TestGatewayWSConfigTagDefaults(t *testing.T) {
	var cfg GatewayWSConfig
	if err := conf.LoadFromYamlBytes([]byte("{}\n"), &cfg); err != nil {
		t.Fatalf("load gateway ws config: %v", err)
	}
	if cfg.AllowQueryToken {
		t.Fatal("query-token auth should be disabled by default")
	}
	if cfg.PingIntervalSeconds != 30 || cfg.HeartbeatTimeoutSeconds != 75 {
		t.Fatalf("gateway ws heartbeat defaults mismatch: %+v", cfg)
	}
	if cfg.CommandRateLimitPerSecond != 20 || cfg.CommandRateLimitBurst != 40 {
		t.Fatalf("gateway ws rate limit defaults mismatch: %+v", cfg)
	}
}

// TestGatewayWSConfigEnvTagsUseBareNames 锁定 #664 决策1：env 覆盖只 wire 裸名，
// 不再有 AGENTS_IM_*/短名别名。env 覆盖语义由 go-zero 内置（proc.Env 进程级缓存使
// 跨用例 t.Setenv 不可靠，故用反射核对 tag 字面量而非真跑 loader）。
func TestGatewayWSConfigEnvTagsUseBareNames(t *testing.T) {
	wantEnv := map[string]string{
		"AllowQueryToken":           "GATEWAY_WS_ALLOW_QUERY_TOKEN",
		"PingIntervalSeconds":       "GATEWAY_WS_PING_INTERVAL_SECONDS",
		"HeartbeatTimeoutSeconds":   "GATEWAY_WS_HEARTBEAT_TIMEOUT_SECONDS",
		"CommandRateLimitPerSecond": "GATEWAY_WS_COMMAND_RATE_LIMIT_PER_SECOND",
		"CommandRateLimitBurst":     "GATEWAY_WS_COMMAND_RATE_LIMIT_BURST",
	}
	typ := reflect.TypeOf(GatewayWSConfig{})
	for field, env := range wantEnv {
		f, ok := typ.FieldByName(field)
		if !ok {
			t.Fatalf("field %s missing", field)
		}
		tag := f.Tag.Get("json")
		if !strings.Contains(tag, "env="+env) {
			t.Fatalf("field %s json tag %q missing env=%s", field, tag, env)
		}
		if strings.Contains(tag, "AGENTS_IM_") || strings.Contains(tag, "_OBS_") {
			t.Fatalf("field %s json tag %q must not keep legacy alias", field, tag)
		}
	}
}

func TestNormalizeGatewayWSConfigNormalizesOrigins(t *testing.T) {
	t.Setenv("GATEWAY_WS_ALLOWED_ORIGINS", "")

	cfg, err := NormalizeGatewayWSConfig(GatewayWSConfig{
		AllowedOrigins: []string{"https://app.example.com", "http://127.0.0.1:5173/", "https://app.example.com"},
	})
	if err != nil {
		t.Fatalf("normalize gateway ws config: %v", err)
	}
	if len(cfg.AllowedOrigins) != 2 || cfg.AllowedOrigins[0] != "https://app.example.com" || cfg.AllowedOrigins[1] != "http://127.0.0.1:5173" {
		t.Fatalf("normalized allowed origins mismatch: %+v", cfg.AllowedOrigins)
	}
}

func TestNormalizeGatewayWSConfigReadsOriginsFromEnv(t *testing.T) {
	t.Setenv("GATEWAY_WS_ALLOWED_ORIGINS", "https://app.example.com, http://127.0.0.1:5173/")

	cfg, err := NormalizeGatewayWSConfig(GatewayWSConfig{})
	if err != nil {
		t.Fatalf("normalize gateway ws config: %v", err)
	}
	if len(cfg.AllowedOrigins) != 2 || cfg.AllowedOrigins[0] != "https://app.example.com" || cfg.AllowedOrigins[1] != "http://127.0.0.1:5173" {
		t.Fatalf("env allowed origins mismatch: %+v", cfg.AllowedOrigins)
	}
}

func TestNormalizeGatewayWSConfigRejectsWildcardOrigin(t *testing.T) {
	t.Setenv("GATEWAY_WS_ALLOWED_ORIGINS", "")

	_, err := NormalizeGatewayWSConfig(GatewayWSConfig{AllowedOrigins: []string{"*"}})
	if err == nil {
		t.Fatal("expected wildcard allowed origin to be rejected")
	}
}

func TestNormalizeGatewayWSConfigFailFastOnPingHeartbeatInversion(t *testing.T) {
	_, err := NormalizeGatewayWSConfig(GatewayWSConfig{
		PingIntervalSeconds:     75,
		HeartbeatTimeoutSeconds: 30,
	})
	if err == nil {
		t.Fatal("expected fail-fast when ping interval >= heartbeat timeout")
	}
}
