package presence

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/wujunhui99/agents_im/internal/config"
)

func TestRedisStoreConnectionLifecycle(t *testing.T) {
	if strings.TrimSpace(os.Getenv("REDIS_ADDR")) == "" {
		t.Skip("REDIS_ADDR is required for Redis presence integration tests")
	}

	redisConfig, err := config.ResolveRedisConfig(config.RedisConfig{})
	if err != nil {
		t.Fatalf("resolve redis config: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	store, err := NewRedisStore(RedisOptions{
		Addr:      redisConfig.Addr,
		Password:  redisConfig.Password,
		DB:        redisConfig.DB,
		KeyPrefix: "agents_im:test:presence:" + strings.ReplaceAll(t.Name(), "/", "_"),
	})
	if err != nil {
		t.Fatalf("new redis store: %v", err)
	}
	defer store.Close()

	if err := store.client.Ping(ctx).Err(); err != nil {
		t.Fatalf("ping redis: %v", err)
	}

	ttl := 5 * time.Second
	metadata := ConnectionMetadata{
		UserID:       "redis_user_1",
		ConnectionID: "redis_conn_1",
		GatewayID:    "gateway_a",
		DeviceID:     "web_device",
		Platform:     "web",
		RemoteAddr:   "127.0.0.1:12345",
		UserAgent:    "presence-test",
	}
	if err := store.RegisterConnection(ctx, metadata, ttl); err != nil {
		t.Fatalf("register connection: %v", err)
	}
	defer store.UnregisterConnection(context.Background(), metadata.UserID, metadata.ConnectionID)

	online, err := store.IsUserOnline(ctx, metadata.UserID)
	if err != nil || !online {
		t.Fatalf("user should be online, online=%v err=%v", online, err)
	}

	connections, err := store.ListUserConnections(ctx, metadata.UserID)
	if err != nil {
		t.Fatal(err)
	}
	if len(connections) != 1 || connections[0].ConnectionID != metadata.ConnectionID || connections[0].GatewayID != metadata.GatewayID {
		t.Fatalf("unexpected connections: %+v", connections)
	}

	lastHeartbeat := connections[0].LastHeartbeatAt
	time.Sleep(time.Millisecond)
	if err := store.Heartbeat(ctx, metadata.UserID, metadata.ConnectionID, ttl); err != nil {
		t.Fatalf("heartbeat: %v", err)
	}
	connections, err = store.ListUserConnections(ctx, metadata.UserID)
	if err != nil {
		t.Fatal(err)
	}
	if len(connections) != 1 || !connections[0].LastHeartbeatAt.After(lastHeartbeat) {
		t.Fatalf("heartbeat did not refresh metadata: before=%s connections=%+v", lastHeartbeat, connections)
	}

	if err := store.UnregisterConnection(ctx, metadata.UserID, metadata.ConnectionID); err != nil {
		t.Fatalf("unregister: %v", err)
	}
	online, err = store.IsUserOnline(ctx, metadata.UserID)
	if err != nil || online {
		t.Fatalf("user should be offline after unregister, online=%v err=%v", online, err)
	}
}
