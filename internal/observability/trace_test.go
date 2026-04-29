package observability

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTraceMiddlewarePropagatesHeadersAndContext(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz?debug=true", nil)
	req.Header.Set(HeaderTraceID, "trace_test_123")
	req.Header.Set(HeaderRequestID, "req_test_123")
	rec := httptest.NewRecorder()

	TraceMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := TraceIDFromContext(r.Context()); got != "trace_test_123" {
			t.Fatalf("trace id not propagated: %q", got)
		}
		w.WriteHeader(http.StatusNoContent)
	})).ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
	if got := rec.Header().Get(HeaderTraceID); got != "trace_test_123" {
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
