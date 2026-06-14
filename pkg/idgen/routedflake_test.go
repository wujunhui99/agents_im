package idgen

import (
	"strconv"
	"sync"
	"testing"
	"time"
)

// decodeFlake splits an id back into its fields for assertions.
func decodeFlake(id int64) (tick, middle, seq int64) {
	seq = id & flakeSequenceMask
	middle = (id >> flakeMiddleShift) & (int64(-1) ^ (int64(-1) << flakeMiddleBits))
	tick = (id >> flakeTimestampShift) & maxFlakeTimestamp
	return
}

func TestRoutedFlakeLayout(t *testing.T) {
	// msg-style config: 1 hint bit (top of middle), 5 machine bits (bottom).
	gen, err := NewRoutedFlake(RoutedFlakeConfig{HintBits: 1, MachineBits: 5, MachineID: 9})
	if err != nil {
		t.Fatalf("NewRoutedFlake: %v", err)
	}
	frozen := time.Date(2026, 6, 14, 0, 0, 0, 0, time.UTC)
	gen.now = func() time.Time { return frozen }

	for _, hint := range []int64{0, 1} {
		id, err := gen.Next(hint)
		if err != nil {
			t.Fatalf("Next(%d): %v", hint, err)
		}
		if id <= 0 {
			t.Fatalf("id %d must be a positive int64 (sign bit 0)", id)
		}
		_, middle, _ := decodeFlake(id)

		// Machine occupies the low 5 bits of the 12-bit middle.
		if got := middle & 0b11111; got != 9 {
			t.Fatalf("machine bits = %d, want 9", got)
		}
		// Hint occupies the single MSB of the middle (bit 11): 100… vs 000….
		if got := (middle >> (flakeMiddleBits - 1)) & 1; got != hint {
			t.Fatalf("hint bit = %d, want %d", got, hint)
		}
		// The reserved gap between hint and machine must be all zero.
		if got := middle & 0b011111100000; got != 0 {
			t.Fatalf("reserved middle bits non-zero: %012b", middle)
		}
	}
}

func TestRoutedFlakeMediaHasNoHint(t *testing.T) {
	// media-style config: 0 hint bits, machine occupies low bits only.
	gen, err := NewRoutedFlake(RoutedFlakeConfig{HintBits: 0, MachineBits: 8, MachineID: 200})
	if err != nil {
		t.Fatalf("NewRoutedFlake: %v", err)
	}
	if _, err := gen.Next(1); err == nil {
		t.Fatal("Next(1) should fail when HintBits == 0")
	}
	id, err := gen.Next(0)
	if err != nil {
		t.Fatalf("Next(0): %v", err)
	}
	_, middle, _ := decodeFlake(id)
	if got := middle & 0xFF; got != 200 {
		t.Fatalf("machine bits = %d, want 200", got)
	}
	// Every bit above the 8 machine bits is reserved and must be 0.
	if got := middle >> 8; got != 0 {
		t.Fatalf("reserved middle bits non-zero: %012b", middle)
	}
}

func TestRoutedFlakeMonotonicWithinTick(t *testing.T) {
	gen, err := NewRoutedFlake(RoutedFlakeConfig{HintBits: 1, MachineBits: 5, MachineID: 1})
	if err != nil {
		t.Fatalf("NewRoutedFlake: %v", err)
	}
	frozen := time.Date(2026, 6, 14, 0, 0, 0, 0, time.UTC)
	gen.now = func() time.Time { return frozen }

	prev := int64(0)
	for i := range 1000 {
		id, err := gen.Next(0)
		if err != nil {
			t.Fatalf("Next #%d: %v", i, err)
		}
		if id <= prev {
			t.Fatalf("id #%d = %d not strictly increasing after %d", i, id, prev)
		}
		prev = id
	}
}

func TestRoutedFlakeSequenceResetsAcrossMs(t *testing.T) {
	gen, err := NewRoutedFlake(RoutedFlakeConfig{HintBits: 1, MachineBits: 5, MachineID: 1})
	if err != nil {
		t.Fatalf("NewRoutedFlake: %v", err)
	}
	base := time.Date(2026, 6, 14, 0, 0, 0, 0, time.UTC)
	gen.now = func() time.Time { return base }

	first, _ := gen.Next(0)
	second, _ := gen.Next(0)
	if _, _, s := decodeFlake(first); s != 0 {
		t.Fatalf("first seq = %d, want 0", s)
	}
	if _, _, s := decodeFlake(second); s != 1 {
		t.Fatalf("second seq = %d, want 1", s)
	}

	gen.now = func() time.Time { return base.Add(time.Millisecond) }
	third, _ := gen.Next(0)
	if _, _, s := decodeFlake(third); s != 0 {
		t.Fatalf("seq after ms advance = %d, want reset to 0", s)
	}
}

func TestRoutedFlakeSequenceExhaustionWaits(t *testing.T) {
	gen, err := NewRoutedFlake(RoutedFlakeConfig{HintBits: 0, MachineBits: 5, MachineID: 1})
	if err != nil {
		t.Fatalf("NewRoutedFlake: %v", err)
	}
	base := time.Date(2026, 6, 14, 0, 0, 0, 0, time.UTC)
	// Clock stays put until the generator is forced to call waitNextMillis; then
	// it observes the advanced clock and continues on the next ms.
	calls := 0
	gen.now = func() time.Time {
		calls++
		if calls > 1025 { // 1 initial read + 1024 ids exhaust the sequence
			return base.Add(time.Millisecond)
		}
		return base
	}

	var last int64
	for i := range 1025 {
		id, err := gen.Next(0)
		if err != nil {
			t.Fatalf("Next #%d: %v", i, err)
		}
		last = id
	}
	// The 1025th id must have rolled to the next ms (tick advanced), not collided.
	tick, _, seq := decodeFlake(last)
	if tick != base.Add(time.Millisecond).UnixMilli()-snowflakeEpochMs {
		t.Fatalf("exhausted sequence did not advance to next ms: tick=%d", tick)
	}
	if seq != 0 {
		t.Fatalf("seq after roll = %d, want 0", seq)
	}
}

func TestRoutedFlakeRejectsClockRollback(t *testing.T) {
	gen, err := NewRoutedFlake(RoutedFlakeConfig{HintBits: 0, MachineBits: 5, MachineID: 1})
	if err != nil {
		t.Fatalf("NewRoutedFlake: %v", err)
	}
	times := []time.Time{
		time.Date(2026, 6, 14, 0, 0, 0, 1_000_000, time.UTC),
		time.Date(2026, 6, 14, 0, 0, 0, 0, time.UTC), // moved backwards 1ms
	}
	gen.now = func() time.Time {
		next := times[0]
		times = times[1:]
		return next
	}

	if _, err := gen.Next(0); err != nil {
		t.Fatalf("first Next: %v", err)
	}
	if _, err := gen.Next(0); err == nil {
		t.Fatal("expected clock rollback to be rejected")
	}
}

func TestRoutedFlakeConcurrentNoDuplicates(t *testing.T) {
	gen, err := NewRoutedFlake(RoutedFlakeConfig{HintBits: 1, MachineBits: 5, MachineID: 3})
	if err != nil {
		t.Fatalf("NewRoutedFlake: %v", err)
	}

	const goroutines = 16
	const perGoroutine = 5000

	var wg sync.WaitGroup
	results := make([][]int64, goroutines)
	for g := range goroutines {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			ids := make([]int64, 0, perGoroutine)
			for range perGoroutine {
				id, err := gen.Next(int64(g & 1))
				if err != nil {
					t.Errorf("Next: %v", err)
					return
				}
				ids = append(ids, id)
			}
			results[g] = ids
		}(g)
	}
	wg.Wait()

	seen := make(map[int64]struct{}, goroutines*perGoroutine)
	for _, ids := range results {
		for _, id := range ids {
			if _, dup := seen[id]; dup {
				t.Fatalf("duplicate id %d", id)
			}
			seen[id] = struct{}{}
		}
	}
	if len(seen) != goroutines*perGoroutine {
		t.Fatalf("got %d unique ids, want %d", len(seen), goroutines*perGoroutine)
	}
}

func TestRoutedFlakeTimeOrdering(t *testing.T) {
	gen, err := NewRoutedFlake(RoutedFlakeConfig{HintBits: 1, MachineBits: 5, MachineID: 7})
	if err != nil {
		t.Fatalf("NewRoutedFlake: %v", err)
	}
	base := time.Date(2026, 6, 14, 0, 0, 0, 0, time.UTC)
	gen.now = func() time.Time { return base }
	// Even with the hint bit set, a later timestamp must sort higher: timestamp
	// dominates the middle segment.
	earlier, _ := gen.Next(1)
	gen.now = func() time.Time { return base.Add(time.Second) }
	later, _ := gen.Next(0)
	if later <= earlier {
		t.Fatalf("later id %d not greater than earlier id %d", later, earlier)
	}
}

func TestNewRoutedFlakeRejectsBadConfig(t *testing.T) {
	cases := []struct {
		name string
		cfg  RoutedFlakeConfig
	}{
		{"no machine bits", RoutedFlakeConfig{HintBits: 1, MachineBits: 0, MachineID: 0}},
		{"middle overflow", RoutedFlakeConfig{HintBits: 6, MachineBits: 7, MachineID: 0}},
		{"machine id too big", RoutedFlakeConfig{HintBits: 0, MachineBits: 4, MachineID: 16}},
		{"machine id negative", RoutedFlakeConfig{HintBits: 0, MachineBits: 4, MachineID: -1}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NewRoutedFlake(tc.cfg); err == nil {
				t.Fatalf("NewRoutedFlake(%+v) accepted bad config", tc.cfg)
			}
		})
	}
}

func TestResolveMachineID(t *testing.T) {
	t.Run("explicit env", func(t *testing.T) {
		t.Setenv(machineIDEnvVar, "42")
		got, err := ResolveMachineID()
		if err != nil {
			t.Fatalf("ResolveMachineID: %v", err)
		}
		if got != 42 {
			t.Fatalf("got %d, want 42", got)
		}
	})

	t.Run("invalid env", func(t *testing.T) {
		t.Setenv(machineIDEnvVar, "not-a-number")
		if _, err := ResolveMachineID(); err == nil {
			t.Fatal("expected error for non-numeric override")
		}
	})

	t.Run("pod ordinal", func(t *testing.T) {
		t.Setenv(machineIDEnvVar, "")
		t.Setenv("POD_NAME", "media-rpc-7")
		got, err := ResolveMachineID()
		if err != nil {
			t.Fatalf("ResolveMachineID: %v", err)
		}
		if got != 7 {
			t.Fatalf("got %d, want 7", got)
		}
	})

	t.Run("no ordinal fails (no hash fallback)", func(t *testing.T) {
		t.Setenv(machineIDEnvVar, "")
		t.Setenv("POD_NAME", "media-rpc-deploy")
		t.Setenv("HOSTNAME", "media-rpc-deploy")
		if _, err := ResolveMachineID(); err == nil {
			t.Fatal("expected error when no ordinal suffix is present")
		}
	})
}

func TestRoutedFlakeNextStringNumeric(t *testing.T) {
	gen, err := NewRoutedFlake(RoutedFlakeConfig{HintBits: 1, MachineBits: 5, MachineID: 1})
	if err != nil {
		t.Fatalf("NewRoutedFlake: %v", err)
	}
	s, err := gen.NextString(1)
	if err != nil {
		t.Fatalf("NextString: %v", err)
	}
	parsed, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		t.Fatalf("NextString returned non-numeric %q: %v", s, err)
	}
	if parsed <= 0 {
		t.Fatalf("id %d must be positive", parsed)
	}
}

func BenchmarkRoutedFlakeNext(b *testing.B) {
	gen, err := NewRoutedFlake(RoutedFlakeConfig{HintBits: 1, MachineBits: 5, MachineID: 1})
	if err != nil {
		b.Fatalf("NewRoutedFlake: %v", err)
	}
	b.ReportAllocs()
	for b.Loop() {
		if _, err := gen.Next(0); err != nil {
			b.Fatalf("Next: %v", err)
		}
	}
}
