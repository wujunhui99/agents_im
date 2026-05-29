package response

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/pkg/observability"
)

func TestWriteJSONSuccessKeepsTraceIDsInHeadersOnly(t *testing.T) {
	rec := httptest.NewRecorder()
	observability.InjectTraceHeaders(rec, observability.TraceContext{TraceID: "4bf92f3577b34da6a3ce929d0e0e4736", RequestID: "req_success_123"})

	WriteOK(rec, map[string]string{"status": "ok"})

	if got := rec.Header().Get(observability.HeaderTraceID); got != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Fatalf("trace header = %q", got)
	}
	if got := rec.Header().Get(observability.HeaderRequestID); got != "req_success_123" {
		t.Fatalf("request header = %q", got)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if _, ok := body["trace_id"]; ok {
		t.Fatalf("success body should not include trace_id: %s", rec.Body.String())
	}
	if _, ok := body["request_id"]; ok {
		t.Fatalf("success body should not include request_id: %s", rec.Body.String())
	}
}

func TestWriteJSONErrorIncludesTraceIDsInHeadersAndBody(t *testing.T) {
	rec := httptest.NewRecorder()
	observability.InjectTraceHeaders(rec, observability.TraceContext{TraceID: "4bf92f3577b34da6a3ce929d0e0e4736", RequestID: "req_error_123"})

	WriteError(rec, apperror.ServiceUnavailable("downstream unavailable"))

	if got := rec.Header().Get(observability.HeaderTraceID); got != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Fatalf("trace header = %q", got)
	}
	if got := rec.Header().Get(observability.HeaderRequestID); got != "req_error_123" {
		t.Fatalf("request header = %q", got)
	}
	var body struct {
		Code      string `json:"code"`
		Message   string `json:"message"`
		TraceID   string `json:"trace_id"`
		RequestID string `json:"request_id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.TraceID != "4bf92f3577b34da6a3ce929d0e0e4736" || body.RequestID != "req_error_123" {
		t.Fatalf("error body trace/request mismatch: %+v body=%s", body, rec.Body.String())
	}
}

func TestGoZeroErrorHandlerCtxIncludesTraceIDsForErrorBody(t *testing.T) {
	ctx := observability.ContextWithTrace(context.Background(), observability.TraceContext{
		TraceID:   "4bf92f3577b34da6a3ce929d0e0e4736",
		RequestID: "req_gozero_123",
	})

	status, payload := GoZeroErrorHandlerCtx(ctx, apperror.Forbidden("forbidden"))

	if status != http.StatusForbidden {
		t.Fatalf("status = %d", status)
	}
	body, ok := payload.(Envelope)
	if !ok {
		t.Fatalf("payload type = %T", payload)
	}
	if body.TraceID != "4bf92f3577b34da6a3ce929d0e0e4736" || body.RequestID != "req_gozero_123" {
		t.Fatalf("go-zero error body trace/request mismatch: %+v", body)
	}
}
