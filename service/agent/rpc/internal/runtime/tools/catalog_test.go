package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/wujunhui99/agents_im/service/agent/rpc/internal/registry"
	"github.com/wujunhui99/agents_im/pkg/model"
)

func TestStaticAdapterCatalogLooksUpAdapterByToolID(t *testing.T) {
	spec := validPythonExecuteToolSpec()
	adapter := fakeAdapter{spec: spec}
	catalog := NewStaticAdapterCatalog(adapter)

	resolved, found, err := catalog.LookupToolAdapter(spec)
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("expected adapter")
	}
	if resolved.Spec().ToolID != spec.ToolID {
		t.Fatalf("adapter mismatch: %+v", resolved.Spec())
	}
}

func TestDefaultLocalAdapterCatalogResolvesPythonExecuteWithDisabledDefault(t *testing.T) {
	ctx := context.Background()
	repo := registry.NewMemoryStore()
	seedPythonExecuteTool(t, ctx, repo, "agent_support", "tool_python")
	resolver, err := NewResolver(repo, WithAdapterCatalog(NewDefaultLocalAdapterCatalog(nil)))
	if err != nil {
		t.Fatal(err)
	}

	resolved, err := resolver.ResolveTool(ctx, ResolveToolRequest{
		AgentID:         "agent_support",
		ToolID:          "tool_python",
		RequireAdapters: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !resolved.HasAdapter() {
		t.Fatal("expected python execute adapter")
	}
	if resolved.Spec.Local == nil || resolved.Spec.Local.HandlerKey != model.LocalToolHandlerPythonExecute {
		t.Fatalf("local python spec mismatch: %+v", resolved.Spec)
	}

	_, err = resolved.Adapter.Invoke(ctx, ToolCall{
		RunID:     "run-123",
		AgentID:   "agent_support",
		ToolID:    "tool_python",
		ToolName:  model.LocalToolHandlerPythonExecute,
		InputJSON: []byte(`{"code":"print(1 + 1)"}`),
		RequestID: "req-123",
	})
	if err == nil || !strings.Contains(err.Error(), "python executor is disabled") {
		t.Fatalf("expected disabled default executor, got %v", err)
	}
}
