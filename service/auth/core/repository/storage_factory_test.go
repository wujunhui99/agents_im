package repository

import (
	"strings"
	"testing"
)

func TestCredentialRepositoryForStorageRequiresExplicitSupportedDriver(t *testing.T) {
	if _, err := NewRepositoryForStorage("sqlite", ""); err == nil || !strings.Contains(err.Error(), "unsupported storage driver") {
		t.Fatalf("unsupported credential repository driver error = %v, want explicit unsupported driver error", err)
	}
}

func TestCredentialRepositoryForStorageAllowsExplicitMemoryDriver(t *testing.T) {
	if _, err := NewRepositoryForStorage("memory", ""); err != nil {
		t.Fatalf("explicit memory credential repository: %v", err)
	}
}
