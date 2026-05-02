package idgen

import (
	"fmt"
	"strconv"
	"sync"
	"time"
)

const (
	defaultEpochMillis = int64(1704067200000) // 2024-01-01T00:00:00Z
	nodeBits           = uint(10)
	sequenceBits       = uint(12)
	maxNodeID          = int64(1<<nodeBits - 1)
	maxSequence        = int64(1<<sequenceBits - 1)
)

type Generator struct {
	mu         sync.Mutex
	nodeID     int64
	epochMilli int64
	lastMilli  int64
	sequence   int64
	now        func() time.Time
}

func New(nodeID int64) (*Generator, error) {
	if nodeID < 0 || nodeID > maxNodeID {
		return nil, fmt.Errorf("node id must be between 0 and %d", maxNodeID)
	}
	return &Generator{
		nodeID:     nodeID,
		epochMilli: defaultEpochMillis,
		now:        time.Now,
	}, nil
}

func (g *Generator) NewString() (string, error) {
	id, err := g.NewInt64()
	if err != nil {
		return "", err
	}
	return strconv.FormatInt(id, 10), nil
}

func (g *Generator) NewInt64() (int64, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	currentMilli := g.now().UTC().UnixMilli()
	if currentMilli < g.lastMilli {
		return 0, fmt.Errorf("clock moved backwards from %d to %d", g.lastMilli, currentMilli)
	}
	if currentMilli == g.lastMilli {
		g.sequence = (g.sequence + 1) & maxSequence
		if g.sequence == 0 {
			currentMilli = g.waitNextMilli(currentMilli)
		}
	} else {
		g.sequence = 0
	}

	g.lastMilli = currentMilli
	id := ((currentMilli - g.epochMilli) << (nodeBits + sequenceBits)) |
		(g.nodeID << sequenceBits) |
		g.sequence
	return id, nil
}

func (g *Generator) waitNextMilli(currentMilli int64) int64 {
	for currentMilli <= g.lastMilli {
		currentMilli = g.now().UTC().UnixMilli()
	}
	return currentMilli
}

var defaultGenerator = mustNewDefault()

func NewString() (string, error) {
	return defaultGenerator.NewString()
}

func mustNewDefault() *Generator {
	g, err := New(1)
	if err != nil {
		panic(err)
	}
	return g
}
