package presence

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestMemoryStoreConnectionLifecycle(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	ttl := time.Minute

	if online, err := store.IsUserOnline(ctx, "user_1"); err != nil || online {
		t.Fatalf("new user should be offline, online=%v err=%v", online, err)
	}

	err := store.RegisterConnection(ctx, ConnectionMetadata{
		UserID:       "user_1",
		ConnectionID: "conn_b",
		GatewayID:    "gateway_1",
		DeviceID:     "ios-device",
		Platform:     "ios",
	}, ttl)
	if err != nil {
		t.Fatalf("register first connection: %v", err)
	}
	if err := store.RegisterConnection(ctx, ConnectionMetadata{UserID: "user_1", ConnectionID: "conn_a"}, ttl); err != nil {
		t.Fatalf("register second connection: %v", err)
	}

	connections, err := store.ListUserConnections(ctx, "user_1")
	if err != nil {
		t.Fatal(err)
	}
	if len(connections) != 2 || connections[0].ConnectionID != "conn_a" || connections[1].ConnectionID != "conn_b" {
		t.Fatalf("connections should be sorted by connection id: %+v", connections)
	}
	if connections[1].InstanceID != "gateway_1" || connections[1].GatewayID != "gateway_1" || connections[1].Platform != "ios" {
		t.Fatalf("connection metadata was not preserved: %+v", connections[1])
	}

	previousHeartbeat := connections[1].LastHeartbeatAt
	time.Sleep(time.Millisecond)
	if err := store.Heartbeat(ctx, "user_1", "conn_b", ttl); err != nil {
		t.Fatalf("heartbeat: %v", err)
	}
	connections, err = store.ListUserConnections(ctx, "user_1")
	if err != nil {
		t.Fatal(err)
	}
	if !connections[1].LastHeartbeatAt.After(previousHeartbeat) {
		t.Fatalf("heartbeat should advance last heartbeat: before=%s after=%s", previousHeartbeat, connections[1].LastHeartbeatAt)
	}

	online, err := store.IsUserOnline(ctx, "user_1")
	if err != nil || !online {
		t.Fatalf("user should be online, online=%v err=%v", online, err)
	}

	if err := store.UnregisterConnection(ctx, "user_1", "conn_a"); err != nil {
		t.Fatalf("unregister first connection: %v", err)
	}
	if err := store.UnregisterConnection(ctx, "user_1", "conn_b"); err != nil {
		t.Fatalf("unregister second connection: %v", err)
	}
	online, err = store.IsUserOnline(ctx, "user_1")
	if err != nil || online {
		t.Fatalf("user should be offline after unregister, online=%v err=%v", online, err)
	}
}

func TestMemoryStoreRejectsInvalidAndExpiredConnections(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()

	if err := store.RegisterConnection(ctx, ConnectionMetadata{UserID: "user_1"}, time.Minute); !errors.Is(err, ErrInvalidConnection) {
		t.Fatalf("expected invalid connection error, got %v", err)
	}
	if err := store.RegisterConnection(ctx, ConnectionMetadata{UserID: "user_1", ConnectionID: "conn_1"}, time.Nanosecond); err != nil {
		t.Fatalf("register short lived connection: %v", err)
	}
	time.Sleep(time.Millisecond)
	if err := store.Heartbeat(ctx, "user_1", "conn_1", time.Minute); !errors.Is(err, ErrConnectionNotFound) {
		t.Fatalf("expected expired connection to be missing, got %v", err)
	}
}

func TestMemoryStoreIsUserOnlineExpiresAfterTTL(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	ttl := 5 * time.Millisecond

	if err := store.RegisterConnection(ctx, ConnectionMetadata{UserID: "user_ttl", ConnectionID: "conn_ttl"}, ttl); err != nil {
		t.Fatalf("register short lived connection: %v", err)
	}
	if online, err := store.IsUserOnline(ctx, "user_ttl"); err != nil || !online {
		t.Fatalf("user should initially be online, online=%v err=%v", online, err)
	}

	time.Sleep(ttl + time.Millisecond)
	online, err := store.IsUserOnline(ctx, "user_ttl")
	if err != nil {
		t.Fatalf("online lookup after ttl: %v", err)
	}
	if online {
		t.Fatal("user should be offline after presence ttl expires")
	}
}
