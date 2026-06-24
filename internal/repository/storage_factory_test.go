package repository

import (
	"strings"
	"testing"
)

func TestRepositoryForStorageRequiresExplicitSupportedDriver(t *testing.T) {
	if _, err := NewAgentAuditRepositoryForStorage("sqlite", ""); err == nil || !strings.Contains(err.Error(), "unsupported storage driver") {
		t.Fatalf("unsupported agent audit repository driver error = %v, want explicit unsupported driver error", err)
	}
}

func TestRepositoryForStorageAllowsExplicitMemoryDriver(t *testing.T) {
	if _, err := NewAgentAuditRepositoryForStorage("memory", ""); err != nil {
		t.Fatalf("explicit memory agent audit repository: %v", err)
	}
}
