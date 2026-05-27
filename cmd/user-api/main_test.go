package main

import (
	"os"
	"strings"
	"testing"
)

func TestMainDelegatesToServiceUserAPIEntry(t *testing.T) {
	source, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}
	content := string(source)

	if !strings.Contains(content, `service/user/api/entry`) {
		t.Fatalf("cmd/user-api must delegate to service/user/api/entry")
	}

	forbiddenImports := []string{
		`internal/auth/repository`,
		`internal/handler`,
		`internal/objectstorage`,
		`internal/repository`,
		`internal/servicecontext/user`,
	}
	for _, forbidden := range forbiddenImports {
		if strings.Contains(content, forbidden) {
			t.Fatalf("cmd/user-api must not own API data/service wiring; found %q", forbidden)
		}
	}
}
