package config

import "testing"

func TestResolveGatewayWSConfigDefaultsAreProductionSafe(t *testing.T) {
	t.Setenv("GATEWAY_WS_ALLOWED_ORIGINS", "")
	t.Setenv("GATEWAY_WS_ALLOW_QUERY_TOKEN", "")
	t.Setenv("GATEWAY_WS_PING_INTERVAL_SECONDS", "")
	t.Setenv("GATEWAY_WS_HEARTBEAT_TIMEOUT_SECONDS", "")
	t.Setenv("GATEWAY_WS_COMMAND_RATE_LIMIT_PER_SECOND", "")
	t.Setenv("GATEWAY_WS_COMMAND_RATE_LIMIT_BURST", "")

	cfg, err := ResolveGatewayWSConfig(GatewayWSConfig{})
	if err != nil {
		t.Fatalf("resolve gateway ws config: %v", err)
	}
	if len(cfg.AllowedOrigins) != 0 {
		t.Fatalf("allowed origins should default empty same-origin policy, got %+v", cfg.AllowedOrigins)
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

func TestResolveGatewayWSConfigFromEnv(t *testing.T) {
	t.Setenv("GATEWAY_WS_ALLOWED_ORIGINS", "https://app.example.com, http://127.0.0.1:5173/")
	t.Setenv("GATEWAY_WS_ALLOW_QUERY_TOKEN", "true")
	t.Setenv("GATEWAY_WS_PING_INTERVAL_SECONDS", "15")
	t.Setenv("GATEWAY_WS_HEARTBEAT_TIMEOUT_SECONDS", "45")
	t.Setenv("GATEWAY_WS_COMMAND_RATE_LIMIT_PER_SECOND", "11")
	t.Setenv("GATEWAY_WS_COMMAND_RATE_LIMIT_BURST", "13")

	cfg, err := ResolveGatewayWSConfig(GatewayWSConfig{})
	if err != nil {
		t.Fatalf("resolve gateway ws env config: %v", err)
	}
	if len(cfg.AllowedOrigins) != 2 || cfg.AllowedOrigins[0] != "https://app.example.com" || cfg.AllowedOrigins[1] != "http://127.0.0.1:5173" {
		t.Fatalf("env allowed origins mismatch: %+v", cfg.AllowedOrigins)
	}
	if !cfg.AllowQueryToken || cfg.PingIntervalSeconds != 15 || cfg.HeartbeatTimeoutSeconds != 45 {
		t.Fatalf("env heartbeat/auth mismatch: %+v", cfg)
	}
	if cfg.CommandRateLimitPerSecond != 11 || cfg.CommandRateLimitBurst != 13 {
		t.Fatalf("env rate limit mismatch: %+v", cfg)
	}
}

func TestResolveGatewayWSConfigRejectsWildcardOrigin(t *testing.T) {
	t.Setenv("GATEWAY_WS_ALLOWED_ORIGINS", "")

	_, err := ResolveGatewayWSConfig(GatewayWSConfig{AllowedOrigins: []string{"*"}})
	if err == nil {
		t.Fatal("expected wildcard allowed origin to be rejected")
	}
}
