package repository

import (
	"strings"
	"testing"
)

func TestRepositoryForStorageRequiresExplicitSupportedDriver(t *testing.T) {
	if _, err := NewRepositoryForStorage("sqlite", ""); err == nil || !strings.Contains(err.Error(), "unsupported storage driver") {
		t.Fatalf("unsupported user repository driver error = %v, want explicit unsupported driver error", err)
	}
	if _, err := NewAgentRepositoryForStorage("sqlite", ""); err == nil || !strings.Contains(err.Error(), "unsupported storage driver") {
		t.Fatalf("unsupported agent repository driver error = %v, want explicit unsupported driver error", err)
	}
	if _, err := NewAgentRegistryRepositoryForStorage("sqlite", ""); err == nil || !strings.Contains(err.Error(), "unsupported storage driver") {
		t.Fatalf("unsupported agent registry repository driver error = %v, want explicit unsupported driver error", err)
	}
	if _, err := NewAgentAuditRepositoryForStorage("sqlite", ""); err == nil || !strings.Contains(err.Error(), "unsupported storage driver") {
		t.Fatalf("unsupported agent audit repository driver error = %v, want explicit unsupported driver error", err)
	}
}

func TestRepositoryForStorageAllowsExplicitMemoryDriver(t *testing.T) {
	if _, err := NewRepositoryForStorage("memory", ""); err != nil {
		t.Fatalf("explicit memory user repository: %v", err)
	}
	if _, err := NewAgentRepositoryForStorage("memory", ""); err != nil {
		t.Fatalf("explicit memory agent repository: %v", err)
	}
	if _, err := NewAgentRegistryRepositoryForStorage("memory", ""); err != nil {
		t.Fatalf("explicit memory agent registry repository: %v", err)
	}
	if _, err := NewAgentAuditRepositoryForStorage("memory", ""); err != nil {
		t.Fatalf("explicit memory agent audit repository: %v", err)
	}
}
