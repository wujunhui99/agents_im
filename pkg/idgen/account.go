package idgen

import (
	"fmt"
	"strconv"
	"sync"
	"time"
)

// D16 typed account ID (00-decisions): Sonyflake-variant int64 layout
//
//	1 bit sign (always 0)
//	39 bits timestamp, 10ms granularity since accountEpoch (~174 years)
//	5 bits account type (0 reserved; extensions go through the allowlist below)
//	9 bits machine id (512 instances)
//	10 bits sequence (1024 ids / 10ms / instance)
//
// The timestamp sits in the high bits so ids stay roughly time-ordered (PK
// locality). The account type is self-describing: "is this receiver an agent"
// is answerable anywhere on the pipeline with zero queries.
//
// Hard rule (D16 解析纪律): account type bits may ONLY be read through
// AccountIDType / helpers in this package — no scattered bit arithmetic in
// services. Only positions that genuinely need the type (msgtransfer toPush
// filtering, agent-rpc trigger final judgment, admin/debug tooling) may branch
// on it.
const (
	accountEpochMs = int64(1704067200000) // 2024-01-01T00:00:00Z, shared with snowflake

	accountTimestampBits = uint(39) // 10ms units
	accountTypeBits      = uint(5)
	accountMachineBits   = uint(9)
	accountSequenceBits  = uint(10)

	accountMachineShift   = accountSequenceBits
	accountTypeShift      = accountSequenceBits + accountMachineBits
	accountTimestampShift = accountSequenceBits + accountMachineBits + accountTypeBits

	maxAccountTimestamp = int64(-1) ^ (int64(-1) << accountTimestampBits)
	maxAccountMachineID = int64(-1) ^ (int64(-1) << accountMachineBits)
	accountSequenceMask = int64(-1) ^ (int64(-1) << accountSequenceBits)
	accountTypeMask     = int64(-1) ^ (int64(-1) << accountTypeBits)
)

// AccountType is the 5-bit account class encoded in every account id.
// 0 is reserved (never issued); new values must be added here (allowlist),
// matching the accounts.account_type column which is created in lockstep and
// immutable for the lifetime of the account.
type AccountType int64

const (
	AccountTypeReserved AccountType = 0
	AccountTypeUser     AccountType = 1
	AccountTypeAgent    AccountType = 2
	AccountTypeAdmin    AccountType = 3
)

// Valid reports whether the type is in the issued allowlist.
func (t AccountType) Valid() bool {
	switch t {
	case AccountTypeUser, AccountTypeAgent, AccountTypeAdmin:
		return true
	default:
		return false
	}
}

func (t AccountType) String() string {
	switch t {
	case AccountTypeUser:
		return "user"
	case AccountTypeAgent:
		return "agent"
	case AccountTypeAdmin:
		return "admin"
	case AccountTypeReserved:
		return "reserved"
	default:
		return fmt.Sprintf("accounttype(%d)", int64(t))
	}
}

// AccountIDType extracts the account type bits from a typed account id.
// It does not (cannot) verify the id was minted by this generator: pre-D16
// legacy ids decode to noise, which is why callers gate type-based behavior
// until the D16 data reset (migration step ①) has happened.
func AccountIDType(id int64) AccountType {
	return AccountType((id >> accountTypeShift) & accountTypeMask)
}

// AccountIDTypeString is AccountIDType for the decimal-string transport form
// (ids cross JSON/JWT as strings — JS Number cannot hold int64). The second
// return is false when the value is not a positive int64 decimal.
func AccountIDTypeString(id string) (AccountType, bool) {
	parsed, err := strconv.ParseInt(id, 10, 64)
	if err != nil || parsed <= 0 {
		return AccountTypeReserved, false
	}
	return AccountIDType(parsed), true
}

// IsAgentAccountID reports whether the decimal-string account id carries the
// agent type bits. Non-numeric / legacy-format ids return false (treated as
// non-agent, the pre-D16 behavior).
func IsAgentAccountID(id string) bool {
	accountType, ok := AccountIDTypeString(id)
	return ok && accountType == AccountTypeAgent
}

// AccountIDGenerator mints typed account ids (account creation path only).
type AccountIDGenerator struct {
	mu        sync.Mutex
	machineID int64
	lastTick  int64
	sequence  int64
	now       func() time.Time
}

func NewAccountIDGenerator(machineID int64) (*AccountIDGenerator, error) {
	if machineID < 0 || machineID > maxAccountMachineID {
		return nil, fmt.Errorf("account id machine id must be between 0 and %d", maxAccountMachineID)
	}
	return &AccountIDGenerator{machineID: machineID, lastTick: -1, now: time.Now}, nil
}

// Next mints one id of the given type.
func (g *AccountIDGenerator) Next(accountType AccountType) (int64, error) {
	if !accountType.Valid() {
		return 0, fmt.Errorf("account type %d is not in the issued allowlist", int64(accountType))
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	tick := g.currentTick()
	if tick < g.lastTick {
		return 0, fmt.Errorf("clock moved backwards while generating account id: last=%d current=%d", g.lastTick, tick)
	}
	if tick == g.lastTick {
		g.sequence = (g.sequence + 1) & accountSequenceMask
		if g.sequence == 0 {
			tick = g.waitNextTick(tick)
		}
	} else {
		g.sequence = 0
	}
	if tick < 0 {
		return 0, fmt.Errorf("clock is before account id epoch")
	}
	if tick > maxAccountTimestamp {
		return 0, fmt.Errorf("account id timestamp overflow: tick=%d", tick)
	}

	g.lastTick = tick
	id := (tick << accountTimestampShift) |
		(int64(accountType) << accountTypeShift) |
		(g.machineID << accountMachineShift) |
		g.sequence
	return id, nil
}

// NextString mints one id in decimal-string transport form.
func (g *AccountIDGenerator) NextString(accountType AccountType) (string, error) {
	id, err := g.Next(accountType)
	if err != nil {
		return "", err
	}
	return strconv.FormatInt(id, 10), nil
}

// currentTick returns 10ms units since the account epoch.
func (g *AccountIDGenerator) currentTick() int64 {
	return (g.now().UTC().UnixMilli() - accountEpochMs) / 10
}

func (g *AccountIDGenerator) waitNextTick(current int64) int64 {
	for {
		tick := g.currentTick()
		if tick > current {
			return tick
		}
		time.Sleep(time.Millisecond)
	}
}
