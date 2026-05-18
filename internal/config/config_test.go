package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
	t.Setenv("KAFKA_BROKERS", "")
	t.Setenv("KAFKA_MESSAGE_EVENTS_TOPIC", "")
	t.Setenv("KAFKA_CONSUMER_GROUP", "")

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
Kafka:
  Brokers: redpanda:9092,localhost:19092
  MessageEventsTopic: message.events.test
  ConsumerGroup: message-transfer-test
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
	if len(cfg.Kafka.Brokers) != 2 || cfg.Kafka.Brokers[0] != "redpanda:9092" || cfg.Kafka.Brokers[1] != "localhost:19092" {
		t.Fatalf("kafka brokers mismatch: %+v", cfg.Kafka.Brokers)
	}
	if cfg.Kafka.MessageEventsTopic != "message.events.test" || cfg.Kafka.ConsumerGroup != "message-transfer-test" {
		t.Fatalf("kafka config mismatch: %+v", cfg.Kafka)
	}
	if len(cfg.MailRPC.Endpoints) != 2 || cfg.MailRPC.Endpoints[0] != "127.0.0.1:9095" || cfg.MailRPC.Endpoints[1] != "mail-rpc:9095" {
		t.Fatalf("mail rpc endpoints mismatch: %+v", cfg.MailRPC.Endpoints)
	}
	if cfg.MailRPC.Timeout != 5000 {
		t.Fatalf("mail rpc timeout = %d, want 5000", cfg.MailRPC.Timeout)
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
	t.Setenv("LANGFUSE_HOST", "https://cloud.langfuse.com")
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
	if cfg.LLMObservability.Langfuse.Host != "https://cloud.langfuse.com" ||
		cfg.LLMObservability.Langfuse.PublicKey != "pk-lf-unit-test" ||
		cfg.LLMObservability.Langfuse.SecretKey != "sk-lf-unit-test" {
		t.Fatalf("langfuse config mismatch: %+v", cfg.LLMObservability.Langfuse)
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
	t.Setenv("KAFKA_BROKERS", "localhost:19092,redpanda:9092")
	t.Setenv("KAFKA_MESSAGE_EVENTS_TOPIC", "message.events.env")
	t.Setenv("KAFKA_CONSUMER_GROUP", "push-worker-env")

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

	kafkaConfig := ResolveKafkaConfig(KafkaConfig{})
	if len(kafkaConfig.Brokers) != 2 || kafkaConfig.Brokers[0] != "localhost:19092" || kafkaConfig.Brokers[1] != "redpanda:9092" {
		t.Fatalf("kafka env brokers mismatch: %+v", kafkaConfig.Brokers)
	}
	if kafkaConfig.MessageEventsTopic != "message.events.env" || kafkaConfig.ConsumerGroup != "push-worker-env" {
		t.Fatalf("kafka env config mismatch: %+v", kafkaConfig)
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

func TestLoadMessageTransferConfigMapsKafkaSettings(t *testing.T) {
	t.Setenv("KAFKA_BROKERS", "")
	t.Setenv("KAFKA_MESSAGE_EVENTS_TOPIC", "")
	t.Setenv("KAFKA_CONSUMER_GROUP", "")
	t.Setenv("MESSAGE_TRANSFER_TOPIC", "")
	t.Setenv("MESSAGE_TRANSFER_CONSUMER_GROUP", "")
	t.Setenv("MESSAGE_TRANSFER_CONSUMER_DRIVER", "")

	configPath := filepath.Join(t.TempDir(), "message-transfer.yaml")
	err := os.WriteFile(configPath, []byte(`
Name: message-transfer-kafka-test
Consumer:
  Driver: kafka
Dispatcher:
  Driver: noop
Kafka:
  Brokers: redpanda:9092, localhost:19092
  MessageEventsTopic: message.events.test
  ConsumerGroup: message-transfer-test
`), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadMessageTransferConfig(configPath)
	if err != nil {
		t.Fatalf("load message transfer config: %v", err)
	}
	if cfg.Consumer.Driver != TransferConsumerKafka {
		t.Fatalf("consumer driver = %q, want kafka", cfg.Consumer.Driver)
	}
	if len(cfg.Kafka.Brokers) != 2 || cfg.Kafka.Brokers[0] != "redpanda:9092" || cfg.Kafka.Brokers[1] != "localhost:19092" {
		t.Fatalf("kafka brokers mismatch: %+v", cfg.Kafka.Brokers)
	}
	if cfg.Consumer.Topic != "message.events.test" || cfg.Consumer.Group != "message-transfer-test" {
		t.Fatalf("consumer topic/group should map from kafka config: %+v", cfg.Consumer)
	}
	if cfg.Kafka.MessageEventsTopic != "message.events.test" || cfg.Kafka.ConsumerGroup != "message-transfer-test" {
		t.Fatalf("kafka config mismatch: %+v", cfg.Kafka)
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
