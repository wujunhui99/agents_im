package token

import (
	"testing"
	"time"
)

func TestIssueProducesUniqueJTIAndRoundTripsDeviceClaims(t *testing.T) {
	m := NewHMACTokenManager("secret", time.Hour)

	raw1, c1, err := m.Issue("usr_1", "alice", "web", "1.2.3.4")
	if err != nil {
		t.Fatalf("issue 1: %v", err)
	}
	_, c2, err := m.Issue("usr_1", "alice", "web", "1.2.3.4")
	if err != nil {
		t.Fatalf("issue 2: %v", err)
	}
	if c1.JTI == "" || c1.JTI == c2.JTI {
		t.Fatalf("jti must be present and unique per issue: %q vs %q", c1.JTI, c2.JTI)
	}

	parsed, err := m.Validate(raw1)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if parsed.UserID != "usr_1" || parsed.Identifier != "alice" {
		t.Fatalf("unexpected subject/identifier: %+v", parsed)
	}
	if parsed.JTI != c1.JTI || parsed.Device != "web" || parsed.LoginIP != "1.2.3.4" {
		t.Fatalf("device claims did not round-trip: %+v", parsed)
	}
}

func TestParseRejectsTamperedSignature(t *testing.T) {
	m := NewHMACTokenManager("secret", time.Hour)
	raw, _, err := m.Issue("usr_1", "alice", "web", "")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if _, err := NewHMACTokenManager("other-secret", time.Hour).Parse(raw); err == nil {
		t.Fatal("expected signature mismatch to be rejected")
	}
}
