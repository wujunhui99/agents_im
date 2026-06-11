package chain

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/wujunhui99/agents_im/pkg/messaging"
)

const (
	dedupTTL    = 7 * 24 * time.Hour
	msgCacheTTL = 24 * time.Hour
	// cacheBucketSize groups 100 seqs per hash bucket (03 §3.3, OpenIM layout).
	cacheBucketSize = 100
)

func dedupKey(senderID, clientMsgID string) string {
	return "msg:dedup:" + senderID + ":" + clientMsgID
}

func cacheBucketKey(conversationID string, seq int64) string {
	return fmt.Sprintf("msg:cache:conv:%s:%d", conversationID, seq/cacheBucketSize)
}

func hasReadKey(conversationID, userID string) string {
	return "msg:hasread:conv:" + conversationID + ":user:" + userID
}

// DedupRecord is what a duplicate client_msg_id resolves to: the seq and
// server_msg_id assigned on first delivery, plus the payload hash for
// idempotency-conflict detection.
type DedupRecord struct {
	ConversationID string `json:"conversation_id"`
	Seq            int64  `json:"seq"`
	ServerMsgID    string `json:"server_msg_id"`
	PayloadHash    string `json:"payload_hash"`
}

// Store bundles the chain's Redis writes: dedup, hot message cache, has-read.
type Store struct {
	rdb *redis.Client
}

func NewStore(rdb *redis.Client) (*Store, error) {
	if rdb == nil {
		return nil, errors.New("chain store requires a redis client")
	}
	return &Store{rdb: rdb}, nil
}

// DedupGet returns the stored record for (sender, client_msg_id), or nil.
func (s *Store) DedupGet(ctx context.Context, senderID, clientMsgID string) (*DedupRecord, error) {
	raw, err := s.rdb.Get(ctx, dedupKey(senderID, clientMsgID)).Result()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("dedup get: %w", err)
	}
	var record DedupRecord
	if err := json.Unmarshal([]byte(raw), &record); err != nil {
		return nil, fmt.Errorf("dedup decode: %w", err)
	}
	return &record, nil
}

// DedupSet stores the record once (NX); a concurrent duplicate keeps the first.
func (s *Store) DedupSet(ctx context.Context, senderID, clientMsgID string, record DedupRecord) error {
	raw, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("dedup encode: %w", err)
	}
	err = s.rdb.SetArgs(ctx, dedupKey(senderID, clientMsgID), raw, redis.SetArgs{Mode: "NX", TTL: dedupTTL}).Err()
	if err != nil && !errors.Is(err, redis.Nil) {
		return fmt.Errorf("dedup set: %w", err)
	}
	return nil
}

// CacheMessages writes seq-assigned events into the hot bucket cache (replayed
// writes overwrite the same hash fields — idempotent).
func (s *Store) CacheMessages(ctx context.Context, conversationID string, events []messaging.MessageEvent) error {
	if len(events) == 0 {
		return nil
	}
	pipe := s.rdb.Pipeline()
	buckets := make(map[string]struct{})
	for _, event := range events {
		raw, err := json.Marshal(event)
		if err != nil {
			return fmt.Errorf("cache encode seq %d: %w", event.Seq, err)
		}
		bucket := cacheBucketKey(conversationID, event.Seq)
		pipe.HSet(ctx, bucket, strconv.FormatInt(event.Seq, 10), raw)
		buckets[bucket] = struct{}{}
	}
	for bucket := range buckets {
		pipe.Expire(ctx, bucket, msgCacheTTL)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("cache messages: %w", err)
	}
	return nil
}

// hasReadAdvance keeps the marker monotonic under replay.
var hasReadAdvance = redis.NewScript(`
local current = tonumber(redis.call('GET', KEYS[1]) or '0')
local next = tonumber(ARGV[1])
if next > current then
  redis.call('SET', KEYS[1], next)
  return next
end
return current
`)

// SetHasRead advances the sender's own read marker to its just-sent seq.
func (s *Store) SetHasRead(ctx context.Context, conversationID, userID string, seq int64) error {
	if err := hasReadAdvance.Run(ctx, s.rdb, []string{hasReadKey(conversationID, userID)}, seq).Err(); err != nil {
		return fmt.Errorf("hasread set: %w", err)
	}
	return nil
}
