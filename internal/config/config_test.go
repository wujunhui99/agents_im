package config

import (
	"errors"
	"os"
	"path/filepath"
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
