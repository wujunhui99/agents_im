package config

import (
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
	if len(cfg.Kafka.Brokers) != 2 || cfg.Kafka.Brokers[0] != "redpanda:9092" || cfg.Kafka.Brokers[1] != "localhost:19092" {
		t.Fatalf("kafka brokers mismatch: %+v", cfg.Kafka.Brokers)
	}
	if cfg.Kafka.MessageEventsTopic != "message.events.test" || cfg.Kafka.ConsumerGroup != "message-transfer-test" {
		t.Fatalf("kafka config mismatch: %+v", cfg.Kafka)
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
	if cfg.Consumer.Driver != TransferConsumerMemory || cfg.Consumer.Topic != "message.accepted.test" || cfg.Consumer.Group != "transfer-test" {
		t.Fatalf("consumer config mismatch: %+v", cfg.Consumer)
	}
	if cfg.Dispatcher.Driver != TransferDispatcherNoop {
		t.Fatalf("dispatcher config mismatch: %+v", cfg.Dispatcher)
	}
	if cfg.Worker.PollIntervalMillis != 25 || cfg.Worker.RetryBackoffMillis != 250 || cfg.Worker.MaxAttempts != 3 {
		t.Fatalf("worker config mismatch: %+v", cfg.Worker)
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
