package main

import (
	"os"
	"strings"
	"testing"
)

func TestMainDelegatesToServiceAgentAPIEntry(t *testing.T) {
	source, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}
	content := string(source)

	if !strings.Contains(content, `service/agent/api/entry`) {
		t.Fatalf("cmd/agent-api must delegate to service/agent/api/entry")
	}

	forbiddenImports := []string{
		`internal/auth/repository`,
		`internal/handler`,
		`internal/repository`,
		`internal/servicecontext/agent`,
	}
	for _, forbidden := range forbiddenImports {
		if strings.Contains(content, forbidden) {
			t.Fatalf("cmd/agent-api must not own API data/service wiring; found %q", forbidden)
		}
	}
}
