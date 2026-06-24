package idgen

import (
	"fmt"
	"hash/fnv"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	snowflakeEpochMs = int64(1767225600000) // 2026-01-01T00:00:00Z
	nodeBits         = uint(10)
	sequenceBits     = uint(12)
	maxNodeID        = int64(-1) ^ (int64(-1) << nodeBits)
	sequenceMask     = int64(-1) ^ (int64(-1) << sequenceBits)
	nodeShift        = sequenceBits
	timestampShift   = sequenceBits + nodeBits
)

type Snowflake struct {
	mu       sync.Mutex
	nodeID   int64
	lastMs   int64
	sequence int64
	now      func() time.Time
}

func NewSnowflake(nodeID int64) (*Snowflake, error) {
	if nodeID < 0 || nodeID > maxNodeID {
		return nil, fmt.Errorf("snowflake node id must be between 0 and %d", maxNodeID)
	}
	return &Snowflake{
		nodeID: nodeID,
		lastMs: -1,
		now:    time.Now,
	}, nil
}

func (g *Snowflake) NextString() (string, error) {
	id, err := g.Next()
	if err != nil {
		return "", err
	}
	return strconv.FormatUint(id, 10), nil
}

func (g *Snowflake) Next() (uint64, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	ms := g.now().UTC().UnixMilli()
	if ms < g.lastMs {
		return 0, fmt.Errorf("clock moved backwards while generating snowflake id: last=%d current=%d", g.lastMs, ms)
	}

	if ms == g.lastMs {
		g.sequence = (g.sequence + 1) & sequenceMask
		if g.sequence == 0 {
			ms = g.waitNextMillis(ms)
		}
	} else {
		g.sequence = 0
	}

	elapsed := ms - snowflakeEpochMs
	if elapsed < 0 {
		return 0, fmt.Errorf("clock is before snowflake epoch")
	}

	g.lastMs = ms
	id := (elapsed << timestampShift) | (g.nodeID << nodeShift) | g.sequence
	return uint64(id), nil
}

func (g *Snowflake) waitNextMillis(currentMs int64) int64 {
	for {
		ms := g.now().UTC().UnixMilli()
		if ms > currentMs {
			return ms
		}
		time.Sleep(time.Millisecond)
	}
}

var defaultSnowflake = mustDefaultSnowflake()

func NewString() (string, error) {
	return defaultSnowflake.NextString()
}

func mustDefaultSnowflake() *Snowflake {
	g, err := NewSnowflake(defaultNodeID())
	if err != nil {
		panic(err)
	}
	return g
}

func defaultNodeID() int64 {
	raw := strings.TrimSpace(os.Getenv("AGENTS_IM_SNOWFLAKE_NODE_ID"))
	if raw != "" {
		value, err := strconv.ParseInt(raw, 10, 64)
		if err == nil && value >= 0 && value <= maxNodeID {
			return value
		}
	}

	host, err := os.Hostname()
	if err != nil || strings.TrimSpace(host) == "" {
		return 0
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(host))
	return int64(h.Sum32() % uint32(maxNodeID+1))
}
