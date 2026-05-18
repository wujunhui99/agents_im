package llmobs

import (
	"context"
	"errors"
	"testing"
)

func TestNewSinkMissingLangfuseConfigIsExplicitlyDisabled(t *testing.T) {
	sink, err := NewSink(Config{
		Enabled: true,
		Backend: BackendLangfuse,
		Langfuse: LangfuseConfig{
			Host: "https://cloud.langfuse.com",
		},
	})
	if !errors.Is(err, ErrLangfuseConfigMissing) {
		t.Fatalf("NewSink error = %v, want %v", err, ErrLangfuseConfigMissing)
	}
	if sink != nil {
		t.Fatalf("missing Langfuse config must not return a sink that can pretend remote export worked: %#v", sink)
	}
}

func TestNoopSinkDoesNotPretendRemoteExportHappened(t *testing.T) {
	sink, err := NewSink(Config{})
	if err != nil {
		t.Fatalf("new default sink: %v", err)
	}
	result, err := sink.Observe(context.Background(), Event{
		Type:      EventTypeRun,
		Status:    StatusSucceeded,
		TraceID:   "trace_noop_1",
		RequestID: "req_noop_1",
	})
	if err != nil {
		t.Fatalf("noop observe: %v", err)
	}
	if result.Exported {
		t.Fatalf("noop sink must not report exported=true: %+v", result)
	}
	if result.Backend != BackendNoop || result.DisabledReason == "" {
		t.Fatalf("noop result should expose disabled backend state: %+v", result)
	}
}

func TestLangfuseLiveExportIsExplicitlyNotImplemented(t *testing.T) {
	sink, err := NewSink(Config{
		Enabled: true,
		Backend: BackendLangfuse,
		Langfuse: LangfuseConfig{
			Host:      "https://cloud.langfuse.com",
			PublicKey: "pk-lf-unit-test",
			SecretKey: "sk-lf-unit-test",
		},
	})
	if !errors.Is(err, ErrLangfuseExportNotImplemented) {
		t.Fatalf("NewSink error = %v, want %v", err, ErrLangfuseExportNotImplemented)
	}
	if sink != nil {
		t.Fatalf("unimplemented Langfuse live export must not return a sink: %#v", sink)
	}
}
