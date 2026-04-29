package presence

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/wujunhui99/agents_im/internal/config"
)

type RedisOptions struct {
	Addr      string
	Password  string
	DB        int
	KeyPrefix string
}

type RedisStore struct {
	client *redis.Client
	keys   redisKeyspace
}

type redisKeyspace struct {
	prefix string
}

func NewRedisStore(options RedisOptions) (*RedisStore, error) {
	if strings.TrimSpace(options.Addr) == "" {
		options.Addr = config.DefaultRedisConfig().Addr
	}
	client := redis.NewClient(&redis.Options{
		Addr:     strings.TrimSpace(options.Addr),
		Password: options.Password,
		DB:       options.DB,
	})
	return NewRedisStoreWithClient(client, options.KeyPrefix), nil
}

func NewRedisStoreWithClient(client *redis.Client, keyPrefix string) *RedisStore {
	return &RedisStore{
		client: client,
		keys:   newRedisKeyspace(keyPrefix),
	}
}

func (s *RedisStore) Close() error {
	if s == nil || s.client == nil {
		return nil
	}
	return s.client.Close()
}

func (s *RedisStore) RegisterConnection(ctx context.Context, metadata ConnectionMetadata, ttl time.Duration) error {
	now := time.Now().UTC()
	metadata, err := normalizeMetadata(metadata, now, ttl)
	if err != nil {
		return err
	}

	pipe := s.client.Pipeline()
	pipe.HSet(ctx, s.keys.connection(metadata.ConnectionID), metadata.redisHash())
	pipe.Expire(ctx, s.keys.connection(metadata.ConnectionID), ttl)
	pipe.SAdd(ctx, s.keys.userConnections(metadata.UserID), metadata.ConnectionID)
	pipe.Expire(ctx, s.keys.userConnections(metadata.UserID), userKeyTTL(ttl))
	pipe.Set(ctx, s.keys.userOnline(metadata.UserID), "1", userKeyTTL(ttl))
	_, err = pipe.Exec(ctx)
	return err
}

func (s *RedisStore) Heartbeat(ctx context.Context, userID string, connectionID string, ttl time.Duration) error {
	if err := validateConnectionRef(userID, connectionID, ttl); err != nil {
		return err
	}
	userID = strings.TrimSpace(userID)
	connectionID = strings.TrimSpace(connectionID)

	fields, err := s.client.HGetAll(ctx, s.keys.connection(connectionID)).Result()
	if err != nil {
		return err
	}
	metadata, ok := metadataFromRedisHash(fields)
	if !ok || metadata.UserID != userID || !metadata.ExpiresAt.After(time.Now().UTC()) {
		return ErrConnectionNotFound
	}

	now := time.Now().UTC()
	metadata.LastHeartbeatAt = now
	metadata.ExpiresAt = now.Add(ttl)
	pipe := s.client.Pipeline()
	pipe.HSet(ctx, s.keys.connection(connectionID), metadata.redisHash())
	pipe.Expire(ctx, s.keys.connection(connectionID), ttl)
	pipe.SAdd(ctx, s.keys.userConnections(userID), connectionID)
	pipe.Expire(ctx, s.keys.userConnections(userID), userKeyTTL(ttl))
	pipe.Set(ctx, s.keys.userOnline(userID), "1", userKeyTTL(ttl))
	_, err = pipe.Exec(ctx)
	return err
}

func (s *RedisStore) UnregisterConnection(ctx context.Context, userID string, connectionID string) error {
	if strings.TrimSpace(userID) == "" || strings.TrimSpace(connectionID) == "" {
		return ErrInvalidConnection
	}
	userID = strings.TrimSpace(userID)
	connectionID = strings.TrimSpace(connectionID)

	pipe := s.client.Pipeline()
	pipe.Del(ctx, s.keys.connection(connectionID))
	pipe.SRem(ctx, s.keys.userConnections(userID), connectionID)
	if _, err := pipe.Exec(ctx); err != nil {
		return err
	}

	connections, err := s.ListUserConnections(ctx, userID)
	if err != nil {
		return err
	}
	if len(connections) == 0 {
		return s.client.Del(ctx, s.keys.userOnline(userID), s.keys.userConnections(userID)).Err()
	}
	return nil
}

func (s *RedisStore) ListUserConnections(ctx context.Context, userID string) ([]ConnectionMetadata, error) {
	if !validateUserID(userID) {
		return nil, ErrInvalidConnection
	}
	userID = strings.TrimSpace(userID)

	connectionIDs, err := s.client.SMembers(ctx, s.keys.userConnections(userID)).Result()
	if err != nil {
		return nil, err
	}
	if len(connectionIDs) == 0 {
		return nil, nil
	}

	now := time.Now().UTC()
	connections := make([]ConnectionMetadata, 0, len(connectionIDs))
	staleConnectionIDs := make([]interface{}, 0)
	staleConnectionKeys := make([]string, 0)
	for _, connectionID := range connectionIDs {
		fields, err := s.client.HGetAll(ctx, s.keys.connection(connectionID)).Result()
		if err != nil {
			return nil, err
		}
		metadata, ok := metadataFromRedisHash(fields)
		if !ok || metadata.UserID != userID || !metadata.ExpiresAt.After(now) {
			staleConnectionIDs = append(staleConnectionIDs, connectionID)
			staleConnectionKeys = append(staleConnectionKeys, s.keys.connection(connectionID))
			continue
		}
		connections = append(connections, metadata)
	}

	if len(staleConnectionIDs) > 0 || len(connections) == 0 {
		pipe := s.client.Pipeline()
		if len(staleConnectionIDs) > 0 {
			pipe.SRem(ctx, s.keys.userConnections(userID), staleConnectionIDs...)
			if len(staleConnectionKeys) > 0 {
				pipe.Del(ctx, staleConnectionKeys...)
			}
		}
		if len(connections) == 0 {
			pipe.Del(ctx, s.keys.userOnline(userID))
		}
		if _, err := pipe.Exec(ctx); err != nil && !errors.Is(err, redis.Nil) {
			return nil, err
		}
	}

	sort.Slice(connections, func(i int, j int) bool {
		return connections[i].ConnectionID < connections[j].ConnectionID
	})
	return connections, nil
}

func (s *RedisStore) IsUserOnline(ctx context.Context, userID string) (bool, error) {
	connections, err := s.ListUserConnections(ctx, userID)
	if err != nil {
		return false, err
	}
	return len(connections) > 0, nil
}

func newRedisKeyspace(prefix string) redisKeyspace {
	prefix = strings.Trim(strings.TrimSpace(prefix), ":")
	if prefix == "" {
		prefix = strings.Trim(config.DefaultPresenceConfig().KeyPrefix, ":")
	}
	return redisKeyspace{prefix: prefix}
}

func (k redisKeyspace) userConnections(userID string) string {
	return k.prefix + ":user:" + userID + ":connections"
}

func (k redisKeyspace) userOnline(userID string) string {
	return k.prefix + ":user:" + userID + ":online"
}

func (k redisKeyspace) connection(connectionID string) string {
	return k.prefix + ":conn:" + connectionID
}

func userKeyTTL(ttl time.Duration) time.Duration {
	if ttl <= 0 {
		return 0
	}
	return ttl * 2
}

func (m ConnectionMetadata) redisHash() map[string]interface{} {
	return map[string]interface{}{
		"user_id":                   m.UserID,
		"connection_id":             m.ConnectionID,
		"gateway_id":                m.GatewayID,
		"device_id":                 m.DeviceID,
		"platform":                  m.Platform,
		"remote_addr":               m.RemoteAddr,
		"user_agent":                m.UserAgent,
		"connected_at_unix_ms":      m.ConnectedAt.UnixMilli(),
		"last_heartbeat_at_unix_ms": m.LastHeartbeatAt.UnixMilli(),
		"expires_at_unix_ms":        m.ExpiresAt.UnixMilli(),
	}
}

func metadataFromRedisHash(fields map[string]string) (ConnectionMetadata, bool) {
	if len(fields) == 0 {
		return ConnectionMetadata{}, false
	}
	connectedAt, err := unixMilliField(fields, "connected_at_unix_ms")
	if err != nil {
		return ConnectionMetadata{}, false
	}
	lastHeartbeatAt, err := unixMilliField(fields, "last_heartbeat_at_unix_ms")
	if err != nil {
		return ConnectionMetadata{}, false
	}
	expiresAt, err := unixMilliField(fields, "expires_at_unix_ms")
	if err != nil {
		return ConnectionMetadata{}, false
	}
	metadata := ConnectionMetadata{
		UserID:          fields["user_id"],
		ConnectionID:    fields["connection_id"],
		GatewayID:       fields["gateway_id"],
		DeviceID:        fields["device_id"],
		Platform:        fields["platform"],
		RemoteAddr:      fields["remote_addr"],
		UserAgent:       fields["user_agent"],
		ConnectedAt:     connectedAt,
		LastHeartbeatAt: lastHeartbeatAt,
		ExpiresAt:       expiresAt,
	}
	return metadata, metadata.UserID != "" && metadata.ConnectionID != ""
}

func unixMilliField(fields map[string]string, name string) (time.Time, error) {
	raw := strings.TrimSpace(fields[name])
	if raw == "" {
		return time.Time{}, strconv.ErrSyntax
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return time.Time{}, err
	}
	return time.UnixMilli(value).UTC(), nil
}
