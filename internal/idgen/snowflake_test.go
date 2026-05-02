package idgen

import (
	"regexp"
	"testing"
	"time"
)

func TestGeneratorNewStringReturnsNumericIDs(t *testing.T) {
	g, err := New(7)
	if err != nil {
		t.Fatal(err)
	}
	fixed := time.Date(2026, 5, 2, 10, 0, 0, 0, time.UTC)
	g.now = func() time.Time { return fixed }

	first, err := g.NewString()
	if err != nil {
		t.Fatal(err)
	}
	second, err := g.NewString()
	if err != nil {
		t.Fatal(err)
	}

	if !regexp.MustCompile(`^[0-9]+$`).MatchString(first) {
		t.Fatalf("id %q must be numeric", first)
	}
	if first == second {
		t.Fatalf("ids must be unique within the same millisecond: %q", first)
	}
}

func TestGeneratorRejectsClockRollback(t *testing.T) {
	g, err := New(1)
	if err != nil {
		t.Fatal(err)
	}
	times := []time.Time{
		time.Date(2026, 5, 2, 10, 0, 0, 1_000_000, time.UTC),
		time.Date(2026, 5, 2, 10, 0, 0, 0, time.UTC),
	}
	g.now = func() time.Time {
		next := times[0]
		times = times[1:]
		return next
	}

	if _, err := g.NewString(); err != nil {
		t.Fatal(err)
	}
	if _, err := g.NewString(); err == nil {
		t.Fatal("expected clock rollback to fail")
	}
}
