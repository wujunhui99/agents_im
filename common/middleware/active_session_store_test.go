package middleware

import (
	"context"
	"testing"
	"time"
)

func TestMemorySessionStoreValidatesActiveJTIPerDevice(t *testing.T) {
	ctx := context.Background()
	s := NewMemorySessionStore()

	if err := s.SetActive(ctx, "usr_1", "web", "jti-web-1", time.Hour); err != nil {
		t.Fatalf("set web: %v", err)
	}
	if err := s.SetActive(ctx, "usr_1", "ios", "jti-ios-1", time.Hour); err != nil {
		t.Fatalf("set ios: %v", err)
	}

	// Both devices have independent active sessions.
	if err := s.Validate(ctx, "usr_1", "web", "jti-web-1"); err != nil {
		t.Fatalf("web active jti should validate: %v", err)
	}
	if err := s.Validate(ctx, "usr_1", "ios", "jti-ios-1"); err != nil {
		t.Fatalf("ios active jti should validate: %v", err)
	}

	// A new web login rotates the web jti; the old one is now rejected, ios untouched.
	if err := s.SetActive(ctx, "usr_1", "web", "jti-web-2", time.Hour); err != nil {
		t.Fatalf("rotate web: %v", err)
	}
	if err := s.Validate(ctx, "usr_1", "web", "jti-web-1"); err == nil {
		t.Fatal("stale web jti must be rejected after rotation")
	}
	if err := s.Validate(ctx, "usr_1", "web", "jti-web-2"); err != nil {
		t.Fatalf("new web jti should validate: %v", err)
	}
	if err := s.Validate(ctx, "usr_1", "ios", "jti-ios-1"); err != nil {
		t.Fatalf("ios session must be unaffected by web rotation: %v", err)
	}
}

func TestMemorySessionStoreRejectsUnknownAndExpired(t *testing.T) {
	ctx := context.Background()
	s := NewMemorySessionStore()

	if err := s.Validate(ctx, "usr_x", "web", "nope"); err == nil {
		t.Fatal("unknown session must be rejected")
	}

	now := time.Now()
	s.now = func() time.Time { return now }
	if err := s.SetActive(ctx, "usr_1", "web", "jti-1", time.Minute); err != nil {
		t.Fatalf("set: %v", err)
	}
	s.now = func() time.Time { return now.Add(2 * time.Minute) }
	if err := s.Validate(ctx, "usr_1", "web", "jti-1"); err == nil {
		t.Fatal("expired session must be rejected")
	}
}
