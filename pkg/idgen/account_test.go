package idgen

import (
	"strconv"
	"testing"
	"time"
)

func TestAccountIDGeneratorEncodesType(t *testing.T) {
	gen, err := NewAccountIDGenerator(7)
	if err != nil {
		t.Fatalf("NewAccountIDGenerator: %v", err)
	}

	for _, accountType := range []AccountType{AccountTypeUser, AccountTypeAgent, AccountTypeAdmin} {
		id, err := gen.Next(accountType)
		if err != nil {
			t.Fatalf("Next(%v): %v", accountType, err)
		}
		if id <= 0 {
			t.Fatalf("Next(%v) = %d, want positive int64", accountType, id)
		}
		if got := AccountIDType(id); got != accountType {
			t.Fatalf("AccountIDType(%d) = %v, want %v", id, got, accountType)
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
	if _, err := gen.Next(AccountTypeReserved); err == nil {
		t.Fatal("Next accepted reserved account type")
	}
	if _, err := gen.Next(AccountType(31)); err == nil {
		t.Fatal("Next accepted account type outside allowlist")
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
		id, err := gen.Next(AccountTypeUser)
		if err != nil {
			t.Fatalf("Next #%d: %v", i, err)
		}
		if id <= previous {
			t.Fatalf("id #%d = %d not strictly increasing after %d", i, id, previous)
		}
		previous = id
	}
}

func TestAccountIDTypeString(t *testing.T) {
	gen, err := NewAccountIDGenerator(3)
	if err != nil {
		t.Fatalf("NewAccountIDGenerator: %v", err)
	}
	agentID, err := gen.NextString(AccountTypeAgent)
	if err != nil {
		t.Fatalf("NextString: %v", err)
	}
	if !IsAgentAccountID(agentID) {
		t.Fatalf("IsAgentAccountID(%q) = false, want true", agentID)
	}

	userID, err := gen.NextString(AccountTypeUser)
	if err != nil {
		t.Fatalf("NextString: %v", err)
	}
	if IsAgentAccountID(userID) {
		t.Fatalf("IsAgentAccountID(%q) = true, want false", userID)
	}

	for _, malformed := range []string{"", "agent_creator", "-5", "0", "9223372036854775808"} {
		if _, ok := AccountIDTypeString(malformed); ok {
			t.Fatalf("AccountIDTypeString(%q) ok = true, want false", malformed)
		}
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
	earlier, err := gen.Next(AccountTypeAdmin)
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	gen.now = func() time.Time { return base.Add(time.Second) }
	later, err := gen.Next(AccountTypeUser)
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	// Timestamp dominates the type bits: later mint sorts higher regardless of type.
	if later <= earlier {
		t.Fatalf("later id %d not greater than earlier id %d", later, earlier)
	}
	if _, err := strconv.ParseInt(strconv.FormatInt(later, 10), 10, 64); err != nil {
		t.Fatalf("decimal round-trip: %v", err)
	}
}
