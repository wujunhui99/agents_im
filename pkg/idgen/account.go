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

// D16 typed account ID (00-decisions): Sonyflake-variant int64 layout.
//
//	 1 bit  sign (always 0 — positive ids keep conversation_id string order +
//	        readable decimal transport)
//	41 bits timestamp, 1ms granularity since accountEpoch (~69.7 years)
//	 3 bits account-facet (see Facet below; agent vs human-side defined today)
//	 9 bits machine id (512 instances)
//	10 bits sequence (1024 ids / ms / instance ≈ 1M ids/s/instance)
//
// The timestamp sits in the high bits so ids stay roughly time-ordered (PK
// locality). The facet is self-describing: "is this receiver an agent" / "does
// this account receive pushes" is answerable anywhere on the pipeline with zero
// queries.
//
// The account-facet is NOT the full account-type enum. It only encodes the
// coarse "human-side vs agent" split that push / trigger judgments need; the
// granular type (admin/test/user/…) lives only in accounts.account_type.
//
// Hard rule (D16 解析纪律): facet bits may ONLY be read through AccountFacet /
// IsAgent / IsAgentAccountID in this package — no scattered bit
// arithmetic in services. Only positions that genuinely need it (msgtransfer
// toPush filtering, agent-rpc trigger final judgment, admin/debug tooling) may
// branch on it.
const (
	// accountEpochMs is 2026-01-01T00:00:00Z, shared with snowflake.go.
	accountEpochMs = int64(1767225600000)

	accountTimestampBits = uint(41) // 1ms units, ~69.7 years
	accountFacetBits     = uint(3)
	accountMachineBits   = uint(9)  // 512 instances
	accountSequenceBits  = uint(10) // 1024 ids / ms / instance

	accountMachineShift   = accountSequenceBits                                         // 10
	accountFacetShift     = accountSequenceBits + accountMachineBits                    // 19
	accountTimestampShift = accountSequenceBits + accountMachineBits + accountFacetBits // 22

	maxAccountTimestamp = int64(-1) ^ (int64(-1) << accountTimestampBits)
	maxAccountMachineID = int64(-1) ^ (int64(-1) << accountMachineBits)
	accountSequenceMask = int64(-1) ^ (int64(-1) << accountSequenceBits)
	accountFacetMask    = int64(-1) ^ (int64(-1) << accountFacetBits)
)

// Facet is the 3-bit account-facet encoded in every account id. Only the
// agent vs human-side split is defined today; bit1-2 are reserved (must be 0)
// and only become issuable through this allowlist. It is kept in lockstep with
// the agent-ness of accounts.account_type at creation time (double-source
// invariant, D16).
type Facet int64

const (
	// FacetAgent: agent account — no connection surface, never in
	// push fanout, is a trigger source.
	FacetAgent Facet = 0b000
	// FacetHuman: human-side account (user / admin / test / … all
	// non-agent types) — has a connection surface, receives pushes.
	FacetHuman Facet = 0b001
)

// Valid reports whether the facet is issuable (reserved bits must be 0).
func (f Facet) Valid() bool {
	return f == FacetAgent || f == FacetHuman
}

// AccountFacet extracts the facet bits from a typed account id. It does not
// (cannot) verify the id was minted by this generator: pre-D16 / non-account
// ids decode to noise, which is why callers gate facet-based behavior until the
// D16 data reset (migration step ①) has happened.
func AccountFacet(id int64) Facet {
	return Facet((id >> accountFacetShift) & accountFacetMask)
}

// IsAgent reports whether the account id carries the agent facet exactly.
func IsAgent(id int64) bool {
	return AccountFacet(id) == FacetAgent
}

// IsAgentAccountID reports whether the decimal-string account id carries the
// agent facet. Non-numeric / non-positive ids return false (treated as
// non-agent, the pre-D16 behavior). This is the string-transport entrypoint
// used by msgtransfer toPush filtering and agent trigger judgment.
func IsAgentAccountID(id string) bool {
	parsed, err := strconv.ParseInt(strings.TrimSpace(id), 10, 64)
	if err != nil || parsed <= 0 {
		return false
	}
	return IsAgent(parsed)
}

// AccountIDGenerator mints typed account ids (account creation path only).
type AccountIDGenerator struct {
	mu        sync.Mutex
	machineID int64
	lastMs    int64
	sequence  int64
	now       func() time.Time
}

func NewAccountIDGenerator(machineID int64) (*AccountIDGenerator, error) {
	if machineID < 0 || machineID > maxAccountMachineID {
		return nil, fmt.Errorf("account id machine id must be between 0 and %d", maxAccountMachineID)
	}
	return &AccountIDGenerator{machineID: machineID, lastMs: -1, now: time.Now}, nil
}

// Next mints one id carrying the given facet.
func (g *AccountIDGenerator) Next(facet Facet) (int64, error) {
	if !facet.Valid() {
		return 0, fmt.Errorf("account facet %d is not issuable (only agent/human)", int64(facet))
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	ms := g.now().UTC().UnixMilli()
	if ms < g.lastMs {
		return 0, fmt.Errorf("clock moved backwards while generating account id: last=%d current=%d", g.lastMs, ms)
	}

	if ms == g.lastMs {
		g.sequence = (g.sequence + 1) & accountSequenceMask
		if g.sequence == 0 {
			ms = g.waitNextMillis(ms)
		}
	} else {
		g.sequence = 0
	}

	tick := ms - accountEpochMs
	if tick < 0 {
		return 0, fmt.Errorf("clock is before account id epoch")
	}
	if tick > maxAccountTimestamp {
		return 0, fmt.Errorf("account id timestamp overflow: tick=%d max=%d", tick, maxAccountTimestamp)
	}

	g.lastMs = ms
	id := (tick << accountTimestampShift) |
		(int64(facet) << accountFacetShift) |
		(g.machineID << accountMachineShift) |
		g.sequence
	return id, nil
}

// NextString mints one id in decimal-string transport form.
func (g *AccountIDGenerator) NextString(facet Facet) (string, error) {
	id, err := g.Next(facet)
	if err != nil {
		return "", err
	}
	return strconv.FormatInt(id, 10), nil
}

func (g *AccountIDGenerator) waitNextMillis(currentMs int64) int64 {
	for {
		ms := g.now().UTC().UnixMilli()
		if ms > currentMs {
			return ms
		}
		time.Sleep(time.Millisecond)
	}
}

var defaultAccountGenerator = mustDefaultAccountGenerator()

// NewAccountString mints a typed account id in decimal-string transport form
// using the process-default generator. Account-creation paths call this with
// the facet that matches the account_type they are about to persist.
func NewAccountString(facet Facet) (string, error) {
	return defaultAccountGenerator.NextString(facet)
}

func mustDefaultAccountGenerator() *AccountIDGenerator {
	g, err := NewAccountIDGenerator(defaultAccountMachineID())
	if err != nil {
		panic(err)
	}
	return g
}

func defaultAccountMachineID() int64 {
	raw := strings.TrimSpace(os.Getenv("AGENTS_IM_SNOWFLAKE_NODE_ID"))
	if raw != "" {
		value, err := strconv.ParseInt(raw, 10, 64)
		if err == nil && value >= 0 && value <= maxAccountMachineID {
			return value
		}
	}

	host, err := os.Hostname()
	if err != nil || strings.TrimSpace(host) == "" {
		return 0
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(host))
	return int64(h.Sum32() % uint32(maxAccountMachineID+1))
}
