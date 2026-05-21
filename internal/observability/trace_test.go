package observability

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/propagation"
)

func TestTraceMiddlewarePropagatesHeadersAndContext(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz?debug=true", nil)
	req.Header.Set(HeaderTraceID, "4bf92f3577b34da6a3ce929d0e0e4736")
	req.Header.Set(HeaderRequestID, "req_test_123")
	rec := httptest.NewRecorder()

	TraceMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := TraceIDFromContext(r.Context()); got != "4bf92f3577b34da6a3ce929d0e0e4736" {
			t.Fatalf("trace id not propagated: %q", got)
		}
		w.WriteHeader(http.StatusNoContent)
	})).ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
	if got := rec.Header().Get(HeaderTraceID); got != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Fatalf("trace response header not set: %q", got)
	}
	if got := rec.Header().Get(HeaderRequestID); got != "req_test_123" {
		t.Fatalf("request response header not set: %q", got)
	}
}

func TestTraceContextFallsBackToTraceparent(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(HeaderTraceparent, "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")

	traceContext := TraceContextFromHTTPRequest(req)

	if traceContext.TraceID != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Fatalf("unexpected trace id from traceparent: %+v", traceContext)
	}
}

func TestNewTraceContextGeneratesCanonicalOTelTraceID(t *testing.T) {
	traceContext := NewTraceContext("", "")

	if !isCanonicalTraceIDForTest(traceContext.TraceID) {
		t.Fatalf("trace id should be canonical OTel hex, got %q", traceContext.TraceID)
	}
	if traceContext.RequestID != traceContext.TraceID {
		t.Fatalf("request id should default to trace id, got %+v", traceContext)
	}
}

func TestTraceContextInjectExtractsW3CPropagation(t *testing.T) {
	parent := "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
	ctx := ContextWithTrace(context.Background(), TraceContext{
		TraceID:     "4bf92f3577b34da6a3ce929d0e0e4736",
		RequestID:   "req_w3c_1",
		TraceParent: parent,
		TraceState:  "rojo=00f067aa0ba902b7",
	})
	carrier := propagation.MapCarrier{}

	InjectTraceContext(ctx, carrier)
	extracted := ExtractTraceContext(context.Background(), carrier)

	if extracted.TraceID != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Fatalf("extracted trace id = %q", extracted.TraceID)
	}
	if extracted.TraceParent != parent {
		t.Fatalf("traceparent not preserved: %+v", extracted)
	}
	if extracted.TraceState != "rojo=00f067aa0ba902b7" {
		t.Fatalf("tracestate not preserved: %+v", extracted)
	}
}

func TestResolveTracingConfigDisabledAndBadEnabledConfig(t *testing.T) {
	disabled, err := ResolveTracingConfig(TracingConfig{}, "message-api")
	if err != nil {
		t.Fatalf("disabled tracing should not require endpoint: %v", err)
	}
	if disabled.Enabled {
		t.Fatalf("tracing should be disabled by default: %+v", disabled)
	}
	if disabled.ServiceName != "message-api" || disabled.Environment == "" {
		t.Fatalf("default service/env mismatch: %+v", disabled)
	}

	_, err = ResolveTracingConfig(TracingConfig{
		Enabled:      true,
		ServiceName:  "message-api",
		OTLPEndpoint: "",
	}, "message-api")
	if err == nil || !strings.Contains(err.Error(), "OTLP endpoint") {
		t.Fatalf("enabled tracing without endpoint should fail visibly, got %v", err)
	}

	_, err = ResolveTracingConfig(TracingConfig{
		Enabled:      true,
		ServiceName:  "message-api",
		OTLPEndpoint: "otel-collector:4317",
		Protocol:     "smtp",
	}, "message-api")
	if err == nil || !strings.Contains(err.Error(), "protocol") {
		t.Fatalf("unsupported tracing protocol should fail visibly, got %v", err)
	}
}

func TestRouteTemplateSanitizesDynamicSegmentsAndSystemRoutes(t *testing.T) {
	cases := map[string]string{
		"/healthz":                             "/healthz",
		"/readyz":                              "/readyz",
		"/metrics":                             "/metrics",
		"/messages/msg_01HXYZ1234567890abcdef": "/messages/:id",
		"/admin/llm-traces/4bf92f3577b34da6a3ce929d0e0e4736": "/admin/llm-traces/:trace_id",
		"/users/1001/friends":                                "/users/:id/friends",
		"/ws":                                                "/ws",
	}
	for input, want := range cases {
		if got := RouteTemplate(input); got != want {
			t.Fatalf("RouteTemplate(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestJaegerTraceURL(t *testing.T) {
	got := JaegerTraceURL("https://jaeger.agenticim.xyz", "4bf92f3577b34da6a3ce929d0e0e4736")
	want := "https://jaeger.agenticim.xyz/trace/4bf92f3577b34da6a3ce929d0e0e4736"
	if got != want {
		t.Fatalf("JaegerTraceURL = %q, want %q", got, want)
	}
	if got := JaegerTraceURL("https://jaeger.agenticim.xyz/", "not a trace"); got != "" {
		t.Fatalf("invalid trace id should not build URL, got %q", got)
	}
}

func isCanonicalTraceIDForTest(value string) bool {
	if len(value) != 32 || value == "00000000000000000000000000000000" {
		return false
	}
	for _, r := range value {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			return false
		}
	}
	return true
}
