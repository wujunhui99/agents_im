package main

import (
	"os"
	"strings"
	"testing"
)

func TestMainDelegatesToServiceAgentAPIEntry(t *testing.T) {
	source, err := os.ReadFile("agent.go")
	if err != nil {
		t.Fatalf("read agent.go: %v", err)
	}
	content := string(source)

	if !strings.Contains(content, `service/agent/api/entry`) {
		t.Fatalf("agent api main must delegate to service/agent/api/entry")
	}

	forbiddenImports := []string{
		`internal/auth/repository`,
		`internal/handler`,
		`internal/repository`,
		`internal/servicecontext/agent`,
	}
	for _, forbidden := range forbiddenImports {
		if strings.Contains(content, forbidden) {
			t.Fatalf("agent api main must not own API data/service wiring; found %q", forbidden)
		}
	}
}
