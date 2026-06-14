package idgen

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// RoutedFlake mints 64-bit snowflake-variant ids in the layout EPIC #527 §0/§1
// standardizes for media_id and msg_id (and, going forward, any entity that
// wants a route-classifiable, machine-stamped, time-ordered id):
//
//	 1 bit  sign        — always 0; positive ids keep decimal-string transport
//	                       in numeric order (same invariant as account_id, D16).
//	41 bits timestamp   — 1ms since flakeEpoch (2026-01-01), ~69.7 years of range.
//	                       High bits ⇒ ids are roughly time-ordered (PK locality).
//	12 bits middle      — a single segment shared by two fields whose boundary is
//	                       DYNAMIC (chosen per generator, not fixed in the layout):
//	                         · route hint occupies the HIGH `hintBits`, growing
//	                           left→right from the middle's MSB;
//	                         · machine id occupies the LOW `machineBits`, growing
//	                           right→left from the middle's LSB;
//	                         · any bits in between are reserved (always 0).
//	                       Because the two fields grow toward each other from
//	                       opposite ends, a deployment can widen the machine field
//	                       (more replicas) or the hint field (more route classes)
//	                       independently, without either field's existing values
//	                       shifting position.
//	10 bits sequence    — 1024 ids / ms / instance ≈ 1M ids/s/instance.
//
// Route hint usage by domain (the layout is identical; only the widths differ):
//   - media: hintBits = 0. media has no single/group semantics, so the hint
//     sub-field is reserved (width 0) and every middle bit above the machine
//     field stays 0. Machine bits are still REQUIRED (see below).
//   - msg:   hintBits = 1. The middle's MSB distinguishes single vs group chat,
//     i.e. `100… vs 000…` at the TOP of the middle segment (NOT `001 vs 000` at
//     the bottom) — so the hint and machine fields never overlap as either grows.
//
// Machine bits are MANDATORY (machineBits ≥ 1): multiple rpc replicas minting in
// the same millisecond would otherwise collide. The machine number MUST be unique
// and stable per instance — derive it from a StatefulSet ordinal, never from a
// hash of the pod name (see ResolveMachineID).
//
// RoutedFlake is a pure library: it is not wired to any domain or process-level
// default here (EPIC #527 #528 non-goal). media-rpc / msg-rpc construct their own
// instance from config + ResolveMachineID.
type RoutedFlake struct {
	mu sync.Mutex

	hintShift   uint  // bit offset of the route-hint sub-field
	hintMax     int64 // largest accepted hint value (0 when hintBits == 0)
	machinePart int64 // machineID already shifted into place (precomputed)

	lastMs   int64
	sequence int64
	now      func() time.Time
}

const (
	flakeTimestampBits = uint(41)
	flakeMiddleBits    = uint(12)
	flakeSequenceBits  = uint(10)

	flakeMiddleShift    = flakeSequenceBits                   // 10
	flakeTimestampShift = flakeSequenceBits + flakeMiddleBits // 22

	maxFlakeTimestamp = int64(-1) ^ (int64(-1) << flakeTimestampBits)
	flakeSequenceMask = int64(-1) ^ (int64(-1) << flakeSequenceBits)
)

// machineIDEnvVar overrides the resolved machine number with an explicit value.
// Distinct from AGENTS_IM_SNOWFLAKE_NODE_ID (the legacy hashing Snowflake) on
// purpose: this generator forbids hash fallback, so it must not share that knob.
const machineIDEnvVar = "AGENTS_IM_SNOWFLAKE_MACHINE_ID"

// RoutedFlakeConfig configures one generator instance.
type RoutedFlakeConfig struct {
	// HintBits is the width of the route-hint sub-field at the high end of the
	// middle segment. 0 reserves it entirely (media). 1 carries single/group
	// for msg.
	HintBits uint
	// MachineBits is the width of the machine-id sub-field at the low end of the
	// middle segment. Must be ≥ 1.
	MachineBits uint
	// MachineID is this instance's machine number, in [0, 2^MachineBits).
	MachineID int64
	// Now overrides the clock (tests only). nil ⇒ time.Now.
	Now func() time.Time
}

// NewRoutedFlake builds a generator from cfg, validating the bit budget and the
// machine id. hintBits + machineBits must fit the 12-bit middle segment.
func NewRoutedFlake(cfg RoutedFlakeConfig) (*RoutedFlake, error) {
	if cfg.MachineBits < 1 {
		return nil, fmt.Errorf("routedflake: machine bits must be >= 1 (machine bits are mandatory to avoid same-ms collisions)")
	}
	if cfg.HintBits+cfg.MachineBits > flakeMiddleBits {
		return nil, fmt.Errorf("routedflake: hint bits (%d) + machine bits (%d) exceed the %d-bit middle segment", cfg.HintBits, cfg.MachineBits, flakeMiddleBits)
	}

	maxMachineID := int64(-1) ^ (int64(-1) << cfg.MachineBits)
	if cfg.MachineID < 0 || cfg.MachineID > maxMachineID {
		return nil, fmt.Errorf("routedflake: machine id %d out of range [0, %d] for %d machine bits", cfg.MachineID, maxMachineID, cfg.MachineBits)
	}

	now := cfg.Now
	if now == nil {
		now = time.Now
	}

	return &RoutedFlake{
		// Hint sits above the reserved gap and the machine field, i.e. it is
		// pinned to the TOP of the middle segment regardless of machineBits.
		hintShift:   flakeMiddleShift + (flakeMiddleBits - cfg.HintBits),
		hintMax:     int64(-1) ^ (int64(-1) << cfg.HintBits),
		machinePart: cfg.MachineID << flakeMiddleShift,
		lastMs:      -1,
		now:         now,
	}, nil
}

// Next mints one id carrying the given route hint. hint must be in
// [0, 2^HintBits); pass 0 when HintBits == 0 (media).
func (g *RoutedFlake) Next(hint int64) (int64, error) {
	if hint < 0 || hint > g.hintMax {
		return 0, fmt.Errorf("routedflake: hint %d out of range [0, %d]", hint, g.hintMax)
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	ms := g.now().UTC().UnixMilli()
	if ms < g.lastMs {
		// Clock moved backwards: refuse rather than risk re-minting ids already
		// issued for [ms, lastMs]. Callers fail fast (no silent fallback).
		return 0, fmt.Errorf("routedflake: clock moved backwards: last=%d current=%d", g.lastMs, ms)
	}

	if ms == g.lastMs {
		g.sequence = (g.sequence + 1) & flakeSequenceMask
		if g.sequence == 0 {
			// Sequence exhausted this ms: spin until the clock advances.
			ms = g.waitNextMillis(ms)
		}
	} else {
		g.sequence = 0
	}

	tick := ms - snowflakeEpochMs
	if tick < 0 {
		return 0, fmt.Errorf("routedflake: clock is before epoch (%d)", snowflakeEpochMs)
	}
	if tick > maxFlakeTimestamp {
		return 0, fmt.Errorf("routedflake: timestamp overflow: tick=%d max=%d", tick, maxFlakeTimestamp)
	}

	g.lastMs = ms
	id := (tick << flakeTimestampShift) |
		(hint << g.hintShift) |
		g.machinePart |
		g.sequence
	return id, nil
}

// NextString mints one id in decimal-string transport form (the wire format per
// the #529 spike: int64 ids exceed JS Number precision, so they cross the wire
// as strings).
func (g *RoutedFlake) NextString(hint int64) (string, error) {
	id, err := g.Next(hint)
	if err != nil {
		return "", err
	}
	return strconv.FormatInt(id, 10), nil
}

func (g *RoutedFlake) waitNextMillis(currentMs int64) int64 {
	for {
		ms := g.now().UTC().UnixMilli()
		if ms > currentMs {
			return ms
		}
		time.Sleep(time.Millisecond)
	}
}

var ordinalSuffix = regexp.MustCompile(`(\d+)$`)

// ResolveMachineID resolves this instance's stable, unique machine number from,
// in priority order:
//
//  1. AGENTS_IM_SNOWFLAKE_MACHINE_ID — explicit override (any deployment may
//     inject the number directly).
//  2. POD_NAME trailing ordinal      — a StatefulSet pod is named
//     `<name>-<ordinal>` (e.g. media-rpc-3 → 3); the ordinal is unique and
//     stable across reschedules, which is exactly what a machine number needs.
//  3. HOSTNAME trailing ordinal      — the OS hostname equals the pod name in a
//     StatefulSet, so it is the same ordinal as a fallback.
//
// It returns an error when none of these yields a non-negative integer ordinal.
// It deliberately NEVER hashes the pod name as a fallback (EPIC #527 §1 / #528):
// a hash can collide, and two replicas with the same machine number silently
// mint duplicate ids. Failing loudly is the point — the caller must supply a
// real ordinal.
func ResolveMachineID() (int64, error) {
	if raw := strings.TrimSpace(os.Getenv(machineIDEnvVar)); raw != "" {
		value, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || value < 0 {
			return 0, fmt.Errorf("routedflake: %s=%q is not a non-negative integer", machineIDEnvVar, raw)
		}
		return value, nil
	}

	// POD_NAME / HOSTNAME are both set to the pod name in a StatefulSet pod;
	// HOSTNAME is the standard one the kubelet injects. We read env (not the
	// os.Hostname() syscall) so the source is explicit and deterministic.
	for _, source := range []string{"POD_NAME", "HOSTNAME"} {
		name := strings.TrimSpace(os.Getenv(source))
		if name == "" {
			continue
		}
		if value, ok := ordinalFromName(name); ok {
			return value, nil
		}
	}

	return 0, fmt.Errorf("routedflake: no stable machine id; set %s or run as a StatefulSet pod with an ordinal suffix (hash fallback is banned)", machineIDEnvVar)
}

func ordinalFromName(name string) (int64, bool) {
	match := ordinalSuffix.FindString(name)
	if match == "" {
		return 0, false
	}
	value, err := strconv.ParseInt(match, 10, 64)
	if err != nil || value < 0 {
		return 0, false
	}
	return value, true
}
