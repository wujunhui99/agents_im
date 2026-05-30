package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wujunhui99/agents_im/pkg/observability"
	"gopkg.in/yaml.v3"
)

func TestLoadAPIConfigResolvesAdminBootstrapFromFileAndEnv(t *testing.T) {
	t.Setenv("ADMIN_BOOTSTRAP_PASSWORD", "unit-test-admin-password")
	configPath := filepath.Join(t.TempDir(), "api.yaml")
	if err := os.WriteFile(configPath, []byte(`
Name: message-api
AdminBootstrap:
  Identifier: amin
  Password: ${ADMIN_BOOTSTRAP_PASSWORD}
  DisplayName: 管理后台管理员
`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadAPIConfig(configPath)
	if err != nil {
		t.Fatalf("load api config: %v", err)
	}
	if cfg.AdminBootstrap.Identifier != "amin" {
		t.Fatalf("admin bootstrap identifier = %q", cfg.AdminBootstrap.Identifier)
	}
	if cfg.AdminBootstrap.Password != "unit-test-admin-password" {
		t.Fatalf("admin bootstrap password was not resolved from env placeholder")
	}
	if cfg.AdminBootstrap.DisplayName != "管理后台管理员" {
		t.Fatalf("admin bootstrap display name = %q", cfg.AdminBootstrap.DisplayName)
	}
}

func TestLoadAPIConfigResolvesAuthSecretFromEnvPlaceholder(t *testing.T) {
	t.Setenv("JWT_ACCESS_SECRET", "unit-test-shared-jwt-secret")
	configPath := filepath.Join(t.TempDir(), "api.yaml")
	if err := os.WriteFile(configPath, []byte(`
Name: message-api
Auth:
  AccessSecret: ${JWT_ACCESS_SECRET}
  AccessExpire: 3600
`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadAPIConfig(configPath)
	if err != nil {
		t.Fatalf("load api config: %v", err)
	}
	if cfg.Auth.AccessSecret != "unit-test-shared-jwt-secret" {
		t.Fatalf("auth access secret was not resolved from env placeholder")
	}
	if cfg.Auth.AccessExpire != 3600 {
		t.Fatalf("auth access expire = %d, want 3600", cfg.Auth.AccessExpire)
	}
}

func TestLoadRPCConfigResolvesAuthSecretFromEnvPlaceholder(t *testing.T) {
	t.Setenv("JWT_ACCESS_SECRET", "unit-test-shared-rpc-jwt-secret")
	configPath := filepath.Join(t.TempDir(), "rpc.yaml")
	if err := os.WriteFile(configPath, []byte(`
Name: message-rpc
Auth:
  AccessSecret: ${JWT_ACCESS_SECRET}
  AccessExpire: 7200
`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadRPCConfig(configPath)
	if err != nil {
		t.Fatalf("load rpc config: %v", err)
	}
	if cfg.Auth.AccessSecret != "unit-test-shared-rpc-jwt-secret" {
		t.Fatalf("rpc auth access secret was not resolved from env placeholder")
	}
	if cfg.Auth.AccessExpire != 7200 {
		t.Fatalf("rpc auth access expire = %d, want 7200", cfg.Auth.AccessExpire)
	}
}

func TestLoadAPIConfigResolvesRedisAndPresenceFromFile(t *testing.T) {
	t.Setenv("REDIS_PASSWORD", "")
	t.Setenv("REDIS_DB", "")
	t.Setenv("PRESENCE_DRIVER", "")
	t.Setenv("PRESENCE_TTL_SECONDS", "")
	t.Setenv("PRESENCE_KEY_PREFIX", "")
	t.Setenv("GATEWAY_WS_ALLOWED_ORIGINS", "")
	t.Setenv("GATEWAY_WS_ALLOW_QUERY_TOKEN", "")
	t.Setenv("GATEWAY_WS_PING_INTERVAL_SECONDS", "")
	t.Setenv("GATEWAY_WS_HEARTBEAT_TIMEOUT_SECONDS", "")
	t.Setenv("GATEWAY_WS_COMMAND_RATE_LIMIT_PER_SECOND", "")
	t.Setenv("GATEWAY_WS_COMMAND_RATE_LIMIT_BURST", "")

	configPath := filepath.Join(t.TempDir(), "api.yaml")
	err := os.WriteFile(configPath, []byte(`
Name: gateway-api
Host: 127.0.0.1
Port: 18888
StorageDriver: postgres
DataSource: ${DATABASE_URL}
Redis:
  Addr: redis.local:6380
  Password: local-dev-only
  DB: 2
Presence:
  Driver: redis
  HeartbeatTTLSeconds: 45
  KeyPrefix: agents_im:test_presence
GatewayWS:
  AllowedOrigins: https://chat.example.com, http://localhost:5173
  AllowQueryToken: true
  PingIntervalSeconds: 10
  HeartbeatTimeoutSeconds: 40
  CommandRateLimitPerSecond: 7
  CommandRateLimitBurst: 9
MailRPC:
  Endpoints: 127.0.0.1:9095,mail-rpc:9095
  Timeout: 5000
`), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadAPIConfig(configPath)
	if err != nil {
		t.Fatalf("load api config: %v", err)
	}
	if cfg.Redis.Addr != "redis.local:6380" || cfg.Redis.Password != "local-dev-only" || cfg.Redis.DB != 2 {
		t.Fatalf("redis config mismatch: %+v", cfg.Redis)
	}
	if cfg.Presence.Driver != PresenceDriverRedis || cfg.Presence.HeartbeatTTLSeconds != 45 || cfg.Presence.KeyPrefix != "agents_im:test_presence" {
		t.Fatalf("presence config mismatch: %+v", cfg.Presence)
	}
	if cfg.StorageDriver != StorageDriverPostgres {
		t.Fatalf("storage driver should remain postgres, got %q", cfg.StorageDriver)
	}
	if len(cfg.GatewayWS.AllowedOrigins) != 2 || cfg.GatewayWS.AllowedOrigins[0] != "https://chat.example.com" || cfg.GatewayWS.AllowedOrigins[1] != "http://localhost:5173" {
		t.Fatalf("gateway websocket allowed origins mismatch: %+v", cfg.GatewayWS.AllowedOrigins)
	}
	if !cfg.GatewayWS.AllowQueryToken || cfg.GatewayWS.PingIntervalSeconds != 10 || cfg.GatewayWS.HeartbeatTimeoutSeconds != 40 {
		t.Fatalf("gateway websocket heartbeat/auth config mismatch: %+v", cfg.GatewayWS)
	}
	if cfg.GatewayWS.CommandRateLimitPerSecond != 7 || cfg.GatewayWS.CommandRateLimitBurst != 9 {
		t.Fatalf("gateway websocket rate limit config mismatch: %+v", cfg.GatewayWS)
	}
	if len(cfg.MailRPC.Endpoints) != 2 || cfg.MailRPC.Endpoints[0] != "127.0.0.1:9095" || cfg.MailRPC.Endpoints[1] != "mail-rpc:9095" {
		t.Fatalf("mail rpc endpoints mismatch: %+v", cfg.MailRPC.Endpoints)
	}
	if cfg.MailRPC.Timeout != 5000 {
		t.Fatalf("mail rpc timeout = %d, want 5000", cfg.MailRPC.Timeout)
	}
}
func TestLoadAPIConfigResolvesTracingFromFile(t *testing.T) {
	clearTracingEnv(t)
	configPath := filepath.Join(t.TempDir(), "message-api.yaml")
	if err := os.WriteFile(configPath, []byte(`
Name: message-api
Tracing:
  Enabled: true
  ServiceName: custom-message-api
  Environment: staging
  OTLPEndpoint: otel-collector.agents-im.svc.cluster.local:4317
  Protocol: grpc
  SamplerRatio: 0.25
  TraceUIBaseURL: https://grafana.agenticim.xyz
  Insecure: true
`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadAPIConfig(configPath)
	if err != nil {
		t.Fatalf("load api config: %v", err)
	}
	if !cfg.Tracing.Enabled ||
		cfg.Tracing.ServiceName != "custom-message-api" ||
		cfg.Tracing.Environment != "staging" ||
		cfg.Tracing.OTLPEndpoint != "otel-collector.agents-im.svc.cluster.local:4317" ||
		cfg.Tracing.Protocol != "grpc" ||
		cfg.Tracing.SamplerRatio != 0.25 ||
		cfg.Tracing.TraceUIBaseURL != "https://grafana.agenticim.xyz" {
		t.Fatalf("tracing config mismatch: %+v", cfg.Tracing)
	}
}

func TestToRestConfMapsTracingToGoZeroTelemetry(t *testing.T) {
	cfg := DefaultAPIConfig()
	cfg.Name = "friends-api"
	cfg.Tracing.Enabled = true
	cfg.Tracing.ServiceName = "friends-api"
	cfg.Tracing.OTLPEndpoint = "otel-collector.agents-im.svc.cluster.local:4317"
	cfg.Tracing.Protocol = "grpc"
	cfg.Tracing.SamplerRatio = 1

	restConf := ToRestConf(cfg)

	if restConf.Telemetry.Name != "friends-api" {
		t.Fatalf("Telemetry.Name = %q", restConf.Telemetry.Name)
	}
	if restConf.Telemetry.Endpoint != "otel-collector.agents-im.svc.cluster.local:4317" {
		t.Fatalf("Telemetry.Endpoint = %q", restConf.Telemetry.Endpoint)
	}
	if restConf.Telemetry.Batcher != "otlpgrpc" {
		t.Fatalf("Telemetry.Batcher = %q", restConf.Telemetry.Batcher)
	}
	if restConf.Telemetry.Sampler != 1 {
		t.Fatalf("Telemetry.Sampler = %v", restConf.Telemetry.Sampler)
	}
	if restConf.Telemetry.Disabled {
		t.Fatalf("Telemetry.Disabled = true")
	}
}

func TestGoZeroTelemetryConfigDisabledWhenTracingDisabled(t *testing.T) {
	telemetry := GoZeroTelemetryConfig(observability.TracingConfig{ServiceName: "friends-api"}, "friends-api")
	if !telemetry.Disabled {
		t.Fatalf("Telemetry.Disabled = false")
	}
	if telemetry.Name != "friends-api" {
		t.Fatalf("Telemetry.Name = %q", telemetry.Name)
	}
}

func TestLoadAPIConfigFailsEnabledTracingWithoutEndpoint(t *testing.T) {
	clearTracingEnv(t)
	configPath := filepath.Join(t.TempDir(), "message-api.yaml")
	if err := os.WriteFile(configPath, []byte(`
Name: message-api
Tracing:
  Enabled: true
  ServiceName: message-api
`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := LoadAPIConfig(configPath)
	if err == nil || !strings.Contains(err.Error(), "OTLP endpoint") {
		t.Fatalf("expected enabled tracing without endpoint to fail, got %v", err)
	}
}

func TestLoadAPIConfigParsesMailRPCYAMLListEndpoints(t *testing.T) {
	clearMailRPCEnv(t)

	configPath := filepath.Join(t.TempDir(), "auth-api.yaml")
	err := os.WriteFile(configPath, []byte(`
Name: auth-api
Host: 127.0.0.1
Port: 18081
MailRPC:
  Endpoints:
    - 127.0.0.1:19095
    - mail-rpc:9095
  Timeout: 5000
`), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadAPIConfig(configPath)
	if err != nil {
		t.Fatalf("load api config: %v", err)
	}
	if len(cfg.MailRPC.Endpoints) != 2 {
		t.Fatalf("mail rpc endpoint count = %d, want 2", len(cfg.MailRPC.Endpoints))
	}
	if cfg.MailRPC.Timeout != 5000 {
		t.Fatalf("mail rpc timeout = %d, want 5000", cfg.MailRPC.Timeout)
	}
}

func TestLoadRPCConfigParsesMailRPCYAMLListEndpoints(t *testing.T) {
	clearMailRPCEnv(t)

	configPath := filepath.Join(t.TempDir(), "auth-rpc.yaml")
	err := os.WriteFile(configPath, []byte(`
Name: auth-rpc
ListenOn: 127.0.0.1:19091
MailRPC:
  Endpoints:
    - 127.0.0.1:19095
    - mail-rpc:9095
  Timeout: 5000
`), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadRPCConfig(configPath)
	if err != nil {
		t.Fatalf("load rpc config: %v", err)
	}
	if len(cfg.MailRPC.Endpoints) != 2 {
		t.Fatalf("mail rpc endpoint count = %d, want 2", len(cfg.MailRPC.Endpoints))
	}
	if cfg.MailRPC.Timeout != 5000 {
		t.Fatalf("mail rpc timeout = %d, want 5000", cfg.MailRPC.Timeout)
	}
}

func TestAuthRPCConfigExamplesUseMailRPCEndpointLists(t *testing.T) {
	for _, path := range []string{
		filepath.Join("..", "..", "etc", "auth-rpc.yaml"),
		filepath.Join("..", "..", "deploy", "k8s", "etc", "auth-rpc.yaml"),
	} {
		t.Run(path, func(t *testing.T) {
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read config: %v", err)
			}
			var data map[string]any
			if err := yaml.Unmarshal(raw, &data); err != nil {
				t.Fatalf("parse config: %v", err)
			}
			mailRPC, ok := data["MailRPC"].(map[string]any)
			if !ok {
				t.Fatalf("MailRPC section is required")
			}
			endpoints, ok := mailRPC["Endpoints"].([]any)
			if !ok || len(endpoints) == 0 {
				t.Fatalf("MailRPC.Endpoints must be a non-empty YAML list")
			}
			for i, endpoint := range endpoints {
				value, ok := endpoint.(string)
				if !ok || strings.TrimSpace(value) == "" {
					t.Fatalf("MailRPC.Endpoints[%d] must be a non-empty string", i)
				}
			}
		})
	}
}

func clearMailRPCEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"AUTH_MAIL_RPC_TARGET",
		"AGENTS_IM_MAIL_RPC_TARGET",
		"MAIL_RPC_TARGET",
		"AUTH_MAIL_RPC_ENDPOINTS",
		"AGENTS_IM_MAIL_RPC_ENDPOINTS",
		"MAIL_RPC_ENDPOINTS",
	} {
		t.Setenv(key, "")
	}
}

func clearPythonExecutorEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"PYTHON_EXECUTOR_BACKEND",
		"AGENTS_IM_PYTHON_EXECUTOR_BACKEND",
		"PYTHON_EXECUTOR_K8S_NAMESPACE",
		"AGENTS_IM_PYTHON_EXECUTOR_K8S_NAMESPACE",
		"PYTHON_EXECUTOR_K8S_IMAGE",
		"AGENTS_IM_PYTHON_EXECUTOR_K8S_IMAGE",
		"PYTHON_EXECUTOR_K8S_SERVICE_ACCOUNT_NAME",
		"AGENTS_IM_PYTHON_EXECUTOR_K8S_SERVICE_ACCOUNT_NAME",
		"PYTHON_EXECUTOR_K8S_RUNTIME_CLASS_NAME",
		"AGENTS_IM_PYTHON_EXECUTOR_K8S_RUNTIME_CLASS_NAME",
		"PYTHON_EXECUTOR_DEFAULT_TIMEOUT_SECONDS",
		"AGENTS_IM_PYTHON_EXECUTOR_DEFAULT_TIMEOUT_SECONDS",
		"PYTHON_EXECUTOR_MAX_TIMEOUT_SECONDS",
		"AGENTS_IM_PYTHON_EXECUTOR_MAX_TIMEOUT_SECONDS",
		"PYTHON_EXECUTOR_DEFAULT_MEMORY_MIB",
		"AGENTS_IM_PYTHON_EXECUTOR_DEFAULT_MEMORY_MIB",
		"PYTHON_EXECUTOR_MAX_MEMORY_MIB",
		"AGENTS_IM_PYTHON_EXECUTOR_MAX_MEMORY_MIB",
		"PYTHON_EXECUTOR_MAX_OUTPUT_BYTES",
		"AGENTS_IM_PYTHON_EXECUTOR_MAX_OUTPUT_BYTES",
	} {
		t.Setenv(key, "")
	}
}

func clearTracingEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"AGENTS_IM_TRACING_ENABLED",
		"TRACING_ENABLED",
		"AGENTS_IM_TRACING_SERVICE_NAME",
		"OTEL_SERVICE_NAME",
		"AGENTS_IM_ENV",
		"OTEL_RESOURCE_ATTRIBUTES",
		"AGENTS_IM_OTLP_ENDPOINT",
		"OTEL_EXPORTER_OTLP_ENDPOINT",
		"OTEL_EXPORTER_OTLP_TRACES_ENDPOINT",
		"AGENTS_IM_OTLP_PROTOCOL",
		"OTEL_EXPORTER_OTLP_PROTOCOL",
		"OTEL_EXPORTER_OTLP_TRACES_PROTOCOL",
		"AGENTS_IM_TRACING_SAMPLER_RATIO",
		"OTEL_TRACES_SAMPLER_ARG",
		"AGENTS_IM_TRACE_UI_BASE_URL",
	} {
		t.Setenv(key, "")
	}
}

func TestLoadAPIConfigResolvesDeepSeekFromFileAndEnv(t *testing.T) {
	t.Setenv("DEEPSEEK_API_KEY", "unit-test-deepseek-api-key")
	t.Setenv("DEEPSEEK_BASE_URL", "")
	t.Setenv("DEEPSEEK_MODEL", "")

	configPath := filepath.Join(t.TempDir(), "agent-api.yaml")
	err := os.WriteFile(configPath, []byte(`
Name: agent-api
DeepSeek:
  APIKey: ${DEEPSEEK_API_KEY}
  BaseURL: https://deepseek.local
  Model: deepseek-local-model
`), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadAPIConfig(configPath)
	if err != nil {
		t.Fatalf("load api config: %v", err)
	}
	if cfg.DeepSeek.APIKey != "unit-test-deepseek-api-key" {
		t.Fatalf("deepseek api key was not resolved from env placeholder")
	}
	if cfg.DeepSeek.BaseURL != "https://deepseek.local" {
		t.Fatalf("deepseek base url = %q", cfg.DeepSeek.BaseURL)
	}
	if cfg.DeepSeek.Model != "deepseek-local-model" {
		t.Fatalf("deepseek model = %q", cfg.DeepSeek.Model)
	}
}

func TestLoadAPIConfigResolvesLLMObservabilityLangfusePlaceholders(t *testing.T) {
	t.Setenv("LLM_OBSERVABILITY_ENABLED", "true")
	t.Setenv("LLM_OBSERVABILITY_BACKEND", "langfuse")
	t.Setenv("LANGFUSE_HOST", "https://langfuse.config-test.local")
	t.Setenv("LANGFUSE_PUBLIC_KEY", "pk-lf-unit-test")
	t.Setenv("LANGFUSE_SECRET_KEY", "sk-lf-unit-test")
	t.Setenv("LLM_OBSERVABILITY_CAPTURE_OUTPUT", "true")

	configPath := filepath.Join(t.TempDir(), "message-api.yaml")
	err := os.WriteFile(configPath, []byte(`
Name: message-api
LLMObservability:
  Enabled: ${LLM_OBSERVABILITY_ENABLED}
  Backend: ${LLM_OBSERVABILITY_BACKEND}
  CaptureOutput: ${LLM_OBSERVABILITY_CAPTURE_OUTPUT}
  Langfuse:
    Host: ${LANGFUSE_HOST}
    PublicKey: ${LANGFUSE_PUBLIC_KEY}
    SecretKey: ${LANGFUSE_SECRET_KEY}
`), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadAPIConfig(configPath)
	if err != nil {
		t.Fatalf("load api config: %v", err)
	}
	if !cfg.LLMObservability.Enabled || cfg.LLMObservability.Backend != LLMObservabilityBackendLangfuse {
		t.Fatalf("llm observability config mismatch: %+v", cfg.LLMObservability)
	}
	if !cfg.LLMObservability.CaptureOutput {
		t.Fatalf("capture output should resolve true: %+v", cfg.LLMObservability)
	}
	if cfg.LLMObservability.Langfuse.Host != "https://langfuse.config-test.local" ||
		cfg.LLMObservability.Langfuse.PublicKey != "pk-lf-unit-test" ||
		cfg.LLMObservability.Langfuse.SecretKey != "sk-lf-unit-test" {
		t.Fatalf("langfuse config mismatch: %+v", cfg.LLMObservability.Langfuse)
	}
}

func TestResolveLLMObservabilityDefaultsLangfuseHost(t *testing.T) {
	t.Setenv("LANGFUSE_HOST", "")
	t.Setenv("LANGFUSE_BASE_URL", "")
	t.Setenv("LANGFUSE_PUBLIC_KEY", "")
	t.Setenv("LANGFUSE_SECRET_KEY", "")

	cfg, err := ResolveLLMObservabilityConfig(LLMObservabilityConfig{})
	if err != nil {
		t.Fatalf("resolve llm observability config: %v", err)
	}
	if cfg.Langfuse.Host != DefaultLangfuseHost {
		t.Fatalf("langfuse host = %q, want %q", cfg.Langfuse.Host, DefaultLangfuseHost)
	}
	if cfg.Enabled || cfg.Backend != LLMObservabilityBackendNoop {
		t.Fatalf("default llm observability should stay disabled noop: %+v", cfg)
	}
}

func TestResolveLLMObservabilityLangfuseHostCanBeOverridden(t *testing.T) {
	t.Setenv("LANGFUSE_HOST", "https://langfuse.override.local")
	t.Setenv("LANGFUSE_BASE_URL", "")

	cfg, err := ResolveLLMObservabilityConfig(DefaultLLMObservabilityConfig())
	if err != nil {
		t.Fatalf("resolve llm observability config: %v", err)
	}
	if cfg.Langfuse.Host != "https://langfuse.override.local" {
		t.Fatalf("langfuse host = %q, want env override", cfg.Langfuse.Host)
	}
}

func TestResolveLLMObservabilityCanEnableLangfuseFromEnv(t *testing.T) {
	t.Setenv("LLM_OBSERVABILITY_ENABLED", "true")
	t.Setenv("LLM_OBSERVABILITY_BACKEND", "langfuse")
	t.Setenv("LLM_OBSERVABILITY_CAPTURE_OUTPUT", "true")
	t.Setenv("LLM_OBSERVABILITY_MAX_OUTPUT_BYTES", "4096")
	t.Setenv("LANGFUSE_HOST", "")
	t.Setenv("LANGFUSE_BASE_URL", "")
	t.Setenv("LANGFUSE_PUBLIC_KEY", "pk-lf-unit-test")
	t.Setenv("LANGFUSE_SECRET_KEY", "sk-lf-unit-test")

	cfg, err := ResolveLLMObservabilityConfig(DefaultLLMObservabilityConfig())
	if err != nil {
		t.Fatalf("resolve llm observability config: %v", err)
	}
	if !cfg.Enabled || cfg.Backend != LLMObservabilityBackendLangfuse {
		t.Fatalf("llm observability should enable langfuse from env: %+v", cfg)
	}
	if !cfg.CaptureOutput || cfg.MaxOutputBytes != 4096 {
		t.Fatalf("capture output settings should resolve from env: %+v", cfg)
	}
	if cfg.Langfuse.Host != DefaultLangfuseHost ||
		cfg.Langfuse.PublicKey != "pk-lf-unit-test" ||
		cfg.Langfuse.SecretKey != "sk-lf-unit-test" {
		t.Fatalf("langfuse env config mismatch: %+v", cfg.Langfuse)
	}
}

func TestResolveLLMObservabilityRejectsUnsupportedBackend(t *testing.T) {
	t.Setenv("LLM_OBSERVABILITY_BACKEND", "langfuze")

	_, err := ResolveLLMObservabilityConfig(DefaultLLMObservabilityConfig())
	if err == nil || !strings.Contains(err.Error(), "unsupported llm observability backend") {
		t.Fatalf("expected unsupported backend error, got %v", err)
	}
}

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

func TestResolvePythonExecutorConfigDefaultsDisabled(t *testing.T) {
	clearPythonExecutorEnv(t)

	cfg, err := ResolvePythonExecutorConfig(PythonExecutorConfig{})
	if err != nil {
		t.Fatalf("resolve python executor config: %v", err)
	}
	if cfg.Backend != PythonExecutorBackendDisabled {
		t.Fatalf("python executor backend = %q, want %q", cfg.Backend, PythonExecutorBackendDisabled)
	}
	if cfg.MaxTimeoutSeconds != 30 || cfg.DefaultTimeoutSeconds != 10 {
		t.Fatalf("python executor timeout defaults mismatch: %+v", cfg)
	}
	if cfg.DefaultMemoryMiB != 256 || cfg.MaxMemoryMiB != 256 {
		t.Fatalf("python executor memory defaults mismatch: %+v", cfg)
	}
	if cfg.MaxOutputBytes != 64*1024 {
		t.Fatalf("python executor max output bytes = %d, want %d", cfg.MaxOutputBytes, 64*1024)
	}
}

func TestResolvePythonExecutorConfigRequiresK8SNamespaceAndImage(t *testing.T) {
	clearPythonExecutorEnv(t)

	_, err := ResolvePythonExecutorConfig(PythonExecutorConfig{Backend: PythonExecutorBackendK8S})
	if err == nil || !strings.Contains(err.Error(), "namespace") || !strings.Contains(err.Error(), "image") {
		t.Fatalf("expected k8s namespace/image validation error, got %v", err)
	}

	cfg, err := ResolvePythonExecutorConfig(PythonExecutorConfig{
		Backend: PythonExecutorBackendK8S,
		K8S: PythonExecutorK8SConfig{
			Namespace: "agent-python-sandbox",
			Image:     "ghcr.io/wujunhui99/agents_im/python-sandbox:test",
		},
	})
	if err != nil {
		t.Fatalf("resolve k8s python executor config: %v", err)
	}
	if cfg.Backend != PythonExecutorBackendK8S || cfg.K8S.Namespace != "agent-python-sandbox" || cfg.K8S.Image == "" {
		t.Fatalf("k8s python executor config mismatch: %+v", cfg)
	}
}

func TestLoadAPIConfigParsesPythonExecutorConfig(t *testing.T) {
	clearPythonExecutorEnv(t)

	configPath := filepath.Join(t.TempDir(), "agent-api.yaml")
	err := os.WriteFile(configPath, []byte(`
Name: agent-api
PythonExecutor:
  Backend: k8s
  DefaultTimeoutSeconds: 5
  MaxTimeoutSeconds: 20
  DefaultMemoryMiB: 128
  MaxMemoryMiB: 256
  MaxOutputBytes: 8192
  K8S:
    Namespace: agent-python-sandbox
    Image: ghcr.io/wujunhui99/agents_im/python-sandbox:test
    ServiceAccountName: python-sandbox-runner
    RuntimeClassName: gvisor
`), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadAPIConfig(configPath)
	if err != nil {
		t.Fatalf("load api config: %v", err)
	}
	if cfg.PythonExecutor.Backend != PythonExecutorBackendK8S {
		t.Fatalf("python executor backend = %q, want k8s", cfg.PythonExecutor.Backend)
	}
	if cfg.PythonExecutor.K8S.Namespace != "agent-python-sandbox" ||
		cfg.PythonExecutor.K8S.Image != "ghcr.io/wujunhui99/agents_im/python-sandbox:test" ||
		cfg.PythonExecutor.K8S.ServiceAccountName != "python-sandbox-runner" ||
		cfg.PythonExecutor.K8S.RuntimeClassName != "gvisor" {
		t.Fatalf("python executor k8s config mismatch: %+v", cfg.PythonExecutor.K8S)
	}
	if cfg.PythonExecutor.DefaultTimeoutSeconds != 5 ||
		cfg.PythonExecutor.MaxTimeoutSeconds != 20 ||
		cfg.PythonExecutor.DefaultMemoryMiB != 128 ||
		cfg.PythonExecutor.MaxMemoryMiB != 256 ||
		cfg.PythonExecutor.MaxOutputBytes != 8192 {
		t.Fatalf("python executor policy defaults mismatch: %+v", cfg.PythonExecutor)
	}
}

func TestResolveDeepSeekConfigUsesDefaultsWithoutKey(t *testing.T) {
	t.Setenv("DEEPSEEK_API_KEY", "")
	t.Setenv("DEEPSEEK_BASE_URL", "")
	t.Setenv("DEEPSEEK_MODEL", "")

	cfg := ResolveDeepSeekConfig(DeepSeekConfig{})
	if cfg.APIKey != "" {
		t.Fatalf("deepseek api key should remain empty when env is unset")
	}
	if cfg.BaseURL != DefaultDeepSeekBaseURL {
		t.Fatalf("deepseek base url = %q, want %q", cfg.BaseURL, DefaultDeepSeekBaseURL)
	}
	if cfg.Model != DefaultDeepSeekModel {
		t.Fatalf("deepseek model = %q, want %q", cfg.Model, DefaultDeepSeekModel)
	}
}

func TestValidateDeepSeekConfigRequiresAPIKey(t *testing.T) {
	t.Setenv("DEEPSEEK_API_KEY", "")
	t.Setenv("DEEPSEEK_BASE_URL", "")
	t.Setenv("DEEPSEEK_MODEL", "")

	cfg := ResolveDeepSeekConfig(DeepSeekConfig{})
	err := ValidateDeepSeekConfig(cfg)
	if !errors.Is(err, ErrDeepSeekAPIKeyMissing) {
		t.Fatalf("validate deepseek config error = %v, want %v", err, ErrDeepSeekAPIKeyMissing)
	}
}

func TestValidateDeepSeekConfigRejectsPlaceholderAPIKey(t *testing.T) {
	t.Setenv("DEEPSEEK_API_KEY", "replace-with-local-deepseek-api-key")
	t.Setenv("DEEPSEEK_BASE_URL", "")
	t.Setenv("DEEPSEEK_MODEL", "")

	cfg := ResolveDeepSeekConfig(DeepSeekConfig{})
	err := ValidateDeepSeekConfig(cfg)
	if !errors.Is(err, ErrDeepSeekAPIKeyPlaceholder) {
		t.Fatalf("validate deepseek config error = %v, want %v", err, ErrDeepSeekAPIKeyPlaceholder)
	}
}

func TestResolveObjectStorageConfigAllowsExternalHTTPSWithInternalHTTP(t *testing.T) {
	t.Setenv("OBJECT_STORAGE_USE_SSL", "")
	t.Setenv("AGENTS_IM_OBJECT_STORAGE_USE_SSL", "")
	t.Setenv("OBJECT_STORAGE_EXTERNAL_USE_SSL", "")
	t.Setenv("AGENTS_IM_OBJECT_STORAGE_EXTERNAL_USE_SSL", "")

	externalUseSSL := true
	cfg, err := ResolveObjectStorageConfig(ObjectStorageConfig{
		Driver:           ObjectStorageDriverMinIO,
		Endpoint:         "127.0.0.1:9000",
		ExternalEndpoint: "storage.example.com",
		Bucket:           "agents-im-media",
		Region:           "us-east-1",
		UseSSL:           false,
		ExternalUseSSL:   &externalUseSSL,
		AccessKeyID:      "unit-test-access-key",
		SecretAccessKey:  "unit-test-secret-key",
	}, StorageDriverPostgres)
	if err != nil {
		t.Fatalf("resolve object storage config: %v", err)
	}
	if cfg.UseSSL {
		t.Fatal("internal object storage UseSSL should remain false")
	}
	if cfg.ExternalUseSSL == nil || !*cfg.ExternalUseSSL {
		t.Fatalf("external presign UseSSL = %v, want true", cfg.ExternalUseSSL)
	}
}

func TestResolveObjectStorageConfigDefaultsExternalEndpointToHTTPSWhenSplitFromInternal(t *testing.T) {
	t.Setenv("OBJECT_STORAGE_USE_SSL", "")
	t.Setenv("AGENTS_IM_OBJECT_STORAGE_USE_SSL", "")
	t.Setenv("OBJECT_STORAGE_EXTERNAL_USE_SSL", "")
	t.Setenv("AGENTS_IM_OBJECT_STORAGE_EXTERNAL_USE_SSL", "")

	cfg, err := ResolveObjectStorageConfig(ObjectStorageConfig{
		Driver:           ObjectStorageDriverMinIO,
		Endpoint:         "127.0.0.1:9000",
		ExternalEndpoint: "storage.example.com",
		Bucket:           "agents-im-media",
		Region:           "us-east-1",
		UseSSL:           false,
		AccessKeyID:      "unit-test-access-key",
		SecretAccessKey:  "unit-test-secret-key",
	}, StorageDriverPostgres)
	if err != nil {
		t.Fatalf("resolve object storage config: %v", err)
	}
	if cfg.UseSSL {
		t.Fatal("internal object storage UseSSL should remain false")
	}
	if cfg.ExternalUseSSL == nil || !*cfg.ExternalUseSSL {
		t.Fatalf("split external object storage endpoint should default ExternalUseSSL to true, got %v", cfg.ExternalUseSSL)
	}
}

func TestLoadAPIConfigResolvesObjectStorageExternalUseSSLFromEnv(t *testing.T) {
	t.Setenv("OBJECT_STORAGE_USE_SSL", "false")
	t.Setenv("OBJECT_STORAGE_EXTERNAL_USE_SSL", "true")

	configPath := filepath.Join(t.TempDir(), "user-api.yaml")
	err := os.WriteFile(configPath, []byte(`
Name: user-api
StorageDriver: postgres
ObjectStorage:
  Driver: minio
  Endpoint: 127.0.0.1:9000
  ExternalEndpoint: storage.example.com
  Bucket: agents-im-media
  Region: us-east-1
  UseSSL: ${OBJECT_STORAGE_USE_SSL}
  ExternalUseSSL: ${OBJECT_STORAGE_EXTERNAL_USE_SSL}
  AccessKeyID: unit-test-access-key
  SecretAccessKey: unit-test-secret-key
`), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadAPIConfig(configPath)
	if err != nil {
		t.Fatalf("load api config: %v", err)
	}
	if cfg.ObjectStorage.UseSSL {
		t.Fatal("internal object storage UseSSL should resolve false from env")
	}
	if cfg.ObjectStorage.ExternalUseSSL == nil || !*cfg.ObjectStorage.ExternalUseSSL {
		t.Fatalf("external object storage UseSSL = %v, want true", cfg.ObjectStorage.ExternalUseSSL)
	}
}

func TestLoadAPIConfigRejectsLoopbackObjectStorageExternalEndpointInProduction(t *testing.T) {
	t.Setenv("AGENTS_IM_ENV", "production")
	t.Setenv("OBJECT_STORAGE_USE_SSL", "")
	t.Setenv("OBJECT_STORAGE_EXTERNAL_USE_SSL", "")

	configPath := filepath.Join(t.TempDir(), "user-api.yaml")
	err := os.WriteFile(configPath, []byte(`
Name: user-api
StorageDriver: postgres
ObjectStorage:
  Driver: minio
  Endpoint: 127.0.0.1:9000
  ExternalEndpoint: 127.0.0.1:9000
  Bucket: agents-im-media
  Region: us-east-1
  UseSSL: false
  AccessKeyID: unit-test-access-key
  SecretAccessKey: unit-test-secret-key
`), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	_, err = LoadAPIConfig(configPath)
	if err == nil {
		t.Fatal("expected production config with loopback object storage external endpoint to fail")
	}
	if !strings.Contains(err.Error(), "object storage external endpoint") || !strings.Contains(err.Error(), "loopback") {
		t.Fatalf("expected loopback external endpoint error, got %v", err)
	}
}

func TestResolveRedisAndPresenceConfigFromEnv(t *testing.T) {
	t.Setenv("REDIS_ADDR", "127.0.0.1:6390")
	t.Setenv("REDIS_PASSWORD", "env-dev-only")
	t.Setenv("REDIS_DB", "3")
	t.Setenv("PRESENCE_DRIVER", "redis")
	t.Setenv("PRESENCE_TTL_SECONDS", "75")
	t.Setenv("PRESENCE_KEY_PREFIX", "agents_im:env_presence")

	redisConfig, err := ResolveRedisConfig(RedisConfig{})
	if err != nil {
		t.Fatalf("resolve redis config: %v", err)
	}
	if redisConfig.Addr != "127.0.0.1:6390" || redisConfig.Password != "env-dev-only" || redisConfig.DB != 3 {
		t.Fatalf("redis env config mismatch: %+v", redisConfig)
	}

	presenceConfig, err := ResolvePresenceConfig(PresenceConfig{})
	if err != nil {
		t.Fatalf("resolve presence config: %v", err)
	}
	if presenceConfig.Driver != PresenceDriverRedis || presenceConfig.HeartbeatTTLSeconds != 75 || presenceConfig.KeyPrefix != "agents_im:env_presence" {
		t.Fatalf("presence env config mismatch: %+v", presenceConfig)
	}

}

func TestLoadMessageTransferConfig(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "message-transfer.yaml")
	err := os.WriteFile(configPath, []byte(`
Name: message-transfer-test
WorkerID: worker-a
DryRun: true
StorageDriver: postgres
DataSource: postgres://example.invalid/agents_im
Consumer:
  Driver: memory
  Topic: message.accepted.test
  Group: transfer-test
Dispatcher:
  Driver: noop
Worker:
  PollIntervalMillis: 25
  RetryBackoffMillis: 250
  MaxAttempts: 3
Observability:
  Enabled: true
  Host: 127.0.0.1
  Port: 18085
`), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadMessageTransferConfig(configPath)
	if err != nil {
		t.Fatalf("load message transfer config: %v", err)
	}
	if cfg.Name != "message-transfer-test" || cfg.WorkerID != "worker-a" || !cfg.DryRun {
		t.Fatalf("basic transfer config mismatch: %+v", cfg)
	}
	if cfg.StorageDriver != StorageDriverPostgres || cfg.DataSource != "postgres://example.invalid/agents_im" {
		t.Fatalf("storage config mismatch: %+v", cfg)
	}
	if cfg.Consumer.Driver != TransferConsumerMemory || cfg.Consumer.Topic != "message.accepted.test" || cfg.Consumer.Group != "transfer-test" {
		t.Fatalf("consumer config mismatch: %+v", cfg.Consumer)
	}
	if cfg.Dispatcher.Driver != TransferDispatcherNoop {
		t.Fatalf("dispatcher config mismatch: %+v", cfg.Dispatcher)
	}
	if cfg.Worker.PollIntervalMillis != 25 || cfg.Worker.RetryBackoffMillis != 250 || cfg.Worker.MaxAttempts != 3 {
		t.Fatalf("worker config mismatch: %+v", cfg.Worker)
	}
	if !cfg.Observability.Enabled || cfg.Observability.Host != "127.0.0.1" || cfg.Observability.Port != 18085 {
		t.Fatalf("observability config mismatch: %+v", cfg.Observability)
	}
}

func TestResolveMessageTransferConsumerDriverSupportsOutbox(t *testing.T) {
	t.Setenv("MESSAGE_TRANSFER_CONSUMER_DRIVER", "")

	for _, value := range []string{"outbox", "postgres_outbox", "postgres-outbox"} {
		if got := ResolveTransferConsumerDriver(value); got != TransferConsumerOutbox {
			t.Fatalf("ResolveTransferConsumerDriver(%q) = %q, want %q", value, got, TransferConsumerOutbox)
		}
	}
}
