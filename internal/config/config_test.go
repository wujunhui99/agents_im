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
