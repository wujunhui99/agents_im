package chain

import (
	"context"
	"errors"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// seqKey is the per-conversation seq arbiter (03 §3.3). Redis owns seq after the
// pipeline switches; PostgreSQL max(messages.seq) only seeds the counter when the
// key is missing (first message ever, or Redis data loss — see B0 deviation note:
// a counter that regresses after AOF loss surfaces as unique-violation alarms in
// the persist consumer, never as silent overwrites).
func seqKey(conversationID string) string {
	return "msg:seq:conv:" + conversationID
}

// MaxSeqQuerier returns the durable max seq for a conversation (0 when none).
type MaxSeqQuerier interface {
	MaxSeq(ctx context.Context, conversationID string) (int64, error)
}

type SeqAllocator struct {
	rdb *redis.Client
	pg  MaxSeqQuerier
}

func NewSeqAllocator(rdb *redis.Client, pg MaxSeqQuerier) (*SeqAllocator, error) {
	if rdb == nil {
		return nil, errors.New("seq allocator requires a redis client")
	}
	if pg == nil {
		return nil, errors.New("seq allocator requires a postgres max-seq querier")
	}
	return &SeqAllocator{rdb: rdb, pg: pg}, nil
}

// Malloc atomically reserves n seqs and returns the first one. Same-conversation
// calls are serialized by the handler's conversation sharding; SETNX keeps the
// seed race-safe regardless.
func (a *SeqAllocator) Malloc(ctx context.Context, conversationID string, n int64) (int64, error) {
	if n <= 0 {
		return 0, fmt.Errorf("seq malloc count must be positive, got %d", n)
	}
	key := seqKey(conversationID)
	exists, err := a.rdb.Exists(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("seq malloc exists %s: %w", conversationID, err)
	}
	if exists == 0 {
		maxSeq, err := a.pg.MaxSeq(ctx, conversationID)
		if err != nil {
			return 0, fmt.Errorf("seq malloc seed from pg %s: %w", conversationID, err)
		}
		err = a.rdb.SetArgs(ctx, key, maxSeq, redis.SetArgs{Mode: "NX"}).Err()
		if err != nil && !errors.Is(err, redis.Nil) {
			return 0, fmt.Errorf("seq malloc seed %s: %w", conversationID, err)
		}
	}
	newMax, err := a.rdb.IncrBy(ctx, key, n).Result()
	if err != nil {
		return 0, fmt.Errorf("seq malloc incr %s: %w", conversationID, err)
	}
	return newMax - n + 1, nil
}
