package middleware

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/pkg/config"
)

// SessionStore keeps the currently-active token id (jti) per (user, device). A new
// login overwrites the entry for that device, so any token carrying an outdated jti
// is rejected — single active session per device.
//
// Layout (one Redis HASH per user): user_active_sessions:{userID} { device_type -> jti }.
type SessionStore interface {
	// SetActive records jti as the active token for (userID, device) with ttl.
	SetActive(ctx context.Context, userID, device, jti string, ttl time.Duration) error
	// Validate succeeds only when jti matches the stored active jti for (userID, device).
	Validate(ctx context.Context, userID, device, jti string) error
}

const sessionKeyPrefix = "user_active_sessions"

func sessionKey(userID string) string {
	return fmt.Sprintf("%s:%s", sessionKeyPrefix, strings.TrimSpace(userID))
}

// RedisSessionStore is the production go-redis/v9 backed SessionStore.
type RedisSessionStore struct {
	client *redis.Client
}

// NewRedisSessionStore builds a store from the shared RedisConfig (same client
// family as pkg/presence, so the YAML stays Addr/Password/DB).
func NewRedisSessionStore(cfg config.RedisConfig) *RedisSessionStore {
	addr := strings.TrimSpace(cfg.Addr)
	if addr == "" {
		addr = config.DefaultRedisConfig().Addr
	}
	return NewRedisSessionStoreWithClient(redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	}))
}

func NewRedisSessionStoreWithClient(client *redis.Client) *RedisSessionStore {
	return &RedisSessionStore{client: client}
}

func (s *RedisSessionStore) Close() error {
	if s.client == nil {
		return nil
	}
	return s.client.Close()
}

func (s *RedisSessionStore) SetActive(ctx context.Context, userID, device, jti string, ttl time.Duration) error {
	if s.client == nil {
		return apperror.Internal("auth session store is required")
	}
	userID = strings.TrimSpace(userID)
	jti = strings.TrimSpace(jti)
	if userID == "" || jti == "" {
		return apperror.InvalidArgument("user_id and jti are required")
	}
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	key := sessionKey(userID)
	pipe := s.client.TxPipeline()
	pipe.HSet(ctx, key, strings.TrimSpace(device), jti)
	pipe.Expire(ctx, key, ttl)
	if _, err := pipe.Exec(ctx); err != nil {
		return apperror.Internal("write active session failed")
	}
	return nil
}

func (s *RedisSessionStore) Validate(ctx context.Context, userID, device, jti string) error {
	if s.client == nil {
		return apperror.Internal("auth session store is required")
	}
	userID = strings.TrimSpace(userID)
	jti = strings.TrimSpace(jti)
	if userID == "" || jti == "" {
		return apperror.Unauthenticated("token session is not active")
	}
	active, err := s.client.HGet(ctx, sessionKey(userID), strings.TrimSpace(device)).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return apperror.Unauthenticated("token session is not active")
		}
		return apperror.Internal("read active session failed")
	}
	if strings.TrimSpace(active) != jti {
		return apperror.Unauthenticated("token session is not active")
	}
	return nil
}
