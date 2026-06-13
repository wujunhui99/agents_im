package idgen

import (
	"strconv"
	"testing"
	"time"
)

func TestAccountIDGeneratorEncodesFacet(t *testing.T) {
	gen, err := NewAccountIDGenerator(7)
	if err != nil {
		t.Fatalf("NewAccountIDGenerator: %v", err)
	}

	for _, facet := range []Facet{FacetHuman, FacetAgent} {
		id, err := gen.Next(facet)
		if err != nil {
			t.Fatalf("Next(%v): %v", facet, err)
		}
		if id <= 0 {
			t.Fatalf("Next(%v) = %d, want positive int64", facet, id)
		}
		if got := AccountFacet(id); got != facet {
			t.Fatalf("AccountFacet(%d) = %v, want %v", id, got, facet)
		}
		if got := ToPush(id); got != facet.ToPush() {
			t.Fatalf("ToPush(%d) = %v, want %v", id, got, facet.ToPush())
		}
		if got := IsAgent(id); got != (facet == FacetAgent) {
			t.Fatalf("IsAgent(%d) = %v, want %v", id, got, facet == FacetAgent)
		}
		if got := (id >> accountMachineShift) & maxAccountMachineID; got != 7 {
			t.Fatalf("machine bits = %d, want 7", got)
		}
	}
}

func TestAccountIDGeneratorRejectsInvalidInput(t *testing.T) {
	if _, err := NewAccountIDGenerator(maxAccountMachineID + 1); err == nil {
		t.Fatal("NewAccountIDGenerator accepted out-of-range machine id")
	}
	gen, err := NewAccountIDGenerator(0)
	if err != nil {
		t.Fatalf("NewAccountIDGenerator: %v", err)
	}
	// Reserved facets (bit1-2 set) are not issuable.
	if _, err := gen.Next(Facet(0b010)); err == nil {
		t.Fatal("Next accepted reserved facet")
	}
	if _, err := gen.Next(Facet(7)); err == nil {
		t.Fatal("Next accepted facet outside allowlist")
	}
}

func TestAccountIDGeneratorMonotonicWithinTick(t *testing.T) {
	gen, err := NewAccountIDGenerator(1)
	if err != nil {
		t.Fatalf("NewAccountIDGenerator: %v", err)
	}
	frozen := time.Date(2026, 6, 12, 0, 0, 0, 0, time.UTC)
	gen.now = func() time.Time { return frozen }

	previous := int64(0)
	for i := range 100 {
		id, err := gen.Next(FacetHuman)
		if err != nil {
			t.Fatalf("Next #%d: %v", i, err)
		}
		if id <= previous {
			t.Fatalf("id #%d = %d not strictly increasing after %d", i, id, previous)
		}
		previous = id
	}
}

func TestIsAgentAccountID(t *testing.T) {
	gen, err := NewAccountIDGenerator(3)
	if err != nil {
		t.Fatalf("NewAccountIDGenerator: %v", err)
	}
	agentID, err := gen.NextString(FacetAgent)
	if err != nil {
		t.Fatalf("NextString: %v", err)
	}
	if !IsAgentAccountID(agentID) {
		t.Fatalf("IsAgentAccountID(%q) = false, want true", agentID)
	}

	humanID, err := gen.NextString(FacetHuman)
	if err != nil {
		t.Fatalf("NextString: %v", err)
	}
	if IsAgentAccountID(humanID) {
		t.Fatalf("IsAgentAccountID(%q) = true, want false", humanID)
	}

	for _, malformed := range []string{"", "agent_creator", "-5", "0", "9223372036854775808"} {
		if IsAgentAccountID(malformed) {
			t.Fatalf("IsAgentAccountID(%q) = true, want false", malformed)
		}
	}
}

func TestAccountIDTimeOrdering(t *testing.T) {
	gen, err := NewAccountIDGenerator(0)
	if err != nil {
		t.Fatalf("NewAccountIDGenerator: %v", err)
	}
	base := time.Date(2026, 6, 12, 0, 0, 0, 0, time.UTC)
	gen.now = func() time.Time { return base }
	earlier, err := gen.Next(FacetAgent)
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	gen.now = func() time.Time { return base.Add(time.Second) }
	later, err := gen.Next(FacetHuman)
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	// Timestamp dominates the facet bits: later mint sorts higher regardless of facet.
	if later <= earlier {
		t.Fatalf("later id %d not greater than earlier id %d", later, earlier)
	}
	if _, err := strconv.ParseInt(strconv.FormatInt(later, 10), 10, 64); err != nil {
		t.Fatalf("decimal round-trip: %v", err)
	}
}
