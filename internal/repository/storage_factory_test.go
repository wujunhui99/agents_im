package repository

import (
	"strings"
	"testing"
)

func TestRepositoryForStorageRequiresExplicitSupportedDriver(t *testing.T) {
	if _, err := NewMessageRepositoryForStorage("sqlite", ""); err == nil || !strings.Contains(err.Error(), "unsupported storage driver") {
		t.Fatalf("unsupported repository driver error = %v, want explicit unsupported driver error", err)
	}
}

func TestRepositoryForStorageAllowsExplicitMemoryDriver(t *testing.T) {
	if _, err := NewMessageRepositoryForStorage("memory", ""); err != nil {
		t.Fatalf("explicit memory repository: %v", err)
	}
}
