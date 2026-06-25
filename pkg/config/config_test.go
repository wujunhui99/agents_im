package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

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
		Driver:           ObjectStorageDriverRustFS,
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

// TestResolveObjectStorageConfigCanonicalizesLegacyDrivers 旧値 minio/s3 归一化到 rustfs（#569 切换期兼容）。
func TestResolveObjectStorageConfigCanonicalizesLegacyDrivers(t *testing.T) {
	for _, legacy := range []string{"minio", "s3", "RustFS"} {
		cfg, err := ResolveObjectStorageConfig(ObjectStorageConfig{
			Driver:          legacy,
			Endpoint:        "127.0.0.1:9000",
			AccessKeyID:     "k",
			SecretAccessKey: "s",
		}, StorageDriverPostgres)
		if err != nil {
			t.Fatalf("driver %q: %v", legacy, err)
		}
		if cfg.Driver != ObjectStorageDriverRustFS {
			t.Fatalf("driver %q canonicalized to %q, want %q", legacy, cfg.Driver, ObjectStorageDriverRustFS)
		}
	}
}

func TestResolveObjectStorageConfigDefaultsExternalEndpointToHTTPSWhenSplitFromInternal(t *testing.T) {
	t.Setenv("OBJECT_STORAGE_USE_SSL", "")
	t.Setenv("AGENTS_IM_OBJECT_STORAGE_USE_SSL", "")
	t.Setenv("OBJECT_STORAGE_EXTERNAL_USE_SSL", "")
	t.Setenv("AGENTS_IM_OBJECT_STORAGE_EXTERNAL_USE_SSL", "")

	cfg, err := ResolveObjectStorageConfig(ObjectStorageConfig{
		Driver:           ObjectStorageDriverRustFS,
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
	configPath := filepath.Join(t.TempDir(), "msgtransfer.yaml")
	err := os.WriteFile(configPath, []byte(`
Name: msgtransfer-test
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
Kafka:
  Enabled: true
  Brokers: redpanda.test:9092,redpanda2.test:9092
  Workers: 4
  Redis:
    Addr: redis.test:6379
    Password: kafka-redis-pass
    DB: 2
`), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadMessageTransferConfig(configPath)
	if err != nil {
		t.Fatalf("load message transfer config: %v", err)
	}
	if cfg.Name != "msgtransfer-test" || cfg.WorkerID != "worker-a" || !cfg.DryRun {
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
	if !cfg.Kafka.Enabled || cfg.Kafka.Brokers != "redpanda.test:9092,redpanda2.test:9092" || cfg.Kafka.Workers != 4 {
		t.Fatalf("kafka config mismatch: %+v", cfg.Kafka)
	}
	if cfg.Kafka.Redis.Addr != "redis.test:6379" || cfg.Kafka.Redis.Password != "kafka-redis-pass" || cfg.Kafka.Redis.DB != 2 {
		t.Fatalf("kafka redis config mismatch: %+v", cfg.Kafka.Redis)
	}
	if got := KafkaBrokerList(cfg.Kafka.Brokers); len(got) != 2 || got[0] != "redpanda.test:9092" {
		t.Fatalf("kafka broker list mismatch: %v", got)
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
