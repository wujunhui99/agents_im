package presence

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/wujunhui99/agents_im/internal/config"
)

var (
	ErrInvalidConnection  = errors.New("presence connection requires user id, connection id, and positive ttl")
	ErrConnectionNotFound = errors.New("presence connection not found")
)

type ConnectionMetadata struct {
	UserID          string
	ConnectionID    string
	InstanceID      string
	GatewayID       string
	DeviceID        string
	Platform        string
	RemoteAddr      string
	UserAgent       string
	ConnectedAt     time.Time
	LastHeartbeatAt time.Time
	ExpiresAt       time.Time
}

type PresenceStore interface {
	RegisterConnection(ctx context.Context, metadata ConnectionMetadata, ttl time.Duration) error
	Heartbeat(ctx context.Context, userID string, connectionID string, ttl time.Duration) error
	UnregisterConnection(ctx context.Context, userID string, connectionID string) error
	ListUserConnections(ctx context.Context, userID string) ([]ConnectionMetadata, error)
	IsUserOnline(ctx context.Context, userID string) (bool, error)
}

func NewStore(presenceConfig config.PresenceConfig, redisConfig config.RedisConfig) (PresenceStore, error) {
	switch config.ResolvePresenceDriver(presenceConfig.Driver) {
	case config.PresenceDriverRedis:
		return NewRedisStore(RedisOptions{
			Addr:      redisConfig.Addr,
			Password:  redisConfig.Password,
			DB:        redisConfig.DB,
			KeyPrefix: presenceConfig.KeyPrefix,
		})
	default:
		return NewMemoryStore(), nil
	}
}

func MustStore(presenceConfig config.PresenceConfig, redisConfig config.RedisConfig) PresenceStore {
	store, err := NewStore(presenceConfig, redisConfig)
	if err != nil {
		panic(err)
	}
	return store
}

func HeartbeatTTL(presenceConfig config.PresenceConfig) time.Duration {
	if presenceConfig.HeartbeatTTLSeconds <= 0 {
		presenceConfig = config.DefaultPresenceConfig()
	}
	return time.Duration(presenceConfig.HeartbeatTTLSeconds) * time.Second
}

func normalizeMetadata(metadata ConnectionMetadata, now time.Time, ttl time.Duration) (ConnectionMetadata, error) {
	if strings.TrimSpace(metadata.UserID) == "" || strings.TrimSpace(metadata.ConnectionID) == "" || ttl <= 0 {
		return ConnectionMetadata{}, ErrInvalidConnection
	}
	metadata.UserID = strings.TrimSpace(metadata.UserID)
	metadata.ConnectionID = strings.TrimSpace(metadata.ConnectionID)
	metadata.InstanceID = strings.TrimSpace(metadata.InstanceID)
	metadata.GatewayID = strings.TrimSpace(metadata.GatewayID)
	if metadata.InstanceID == "" {
		metadata.InstanceID = metadata.GatewayID
	}
	if metadata.GatewayID == "" {
		metadata.GatewayID = metadata.InstanceID
	}
	if metadata.ConnectedAt.IsZero() {
		metadata.ConnectedAt = now
	}
	if metadata.LastHeartbeatAt.IsZero() {
		metadata.LastHeartbeatAt = now
	}
	metadata.ExpiresAt = now.Add(ttl)
	return metadata, nil
}

func validateConnectionRef(userID string, connectionID string, ttl time.Duration) error {
	if strings.TrimSpace(userID) == "" || strings.TrimSpace(connectionID) == "" || ttl <= 0 {
		return ErrInvalidConnection
	}
	return nil
}

func validateUserID(userID string) bool {
	return strings.TrimSpace(userID) != ""
}
