package response

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/wujunhui99/agents_im/internal/apperror"
	"github.com/wujunhui99/agents_im/internal/observability"
)

type Envelope struct {
	Code      string      `json:"code"`
	Message   string      `json:"message"`
	Data      interface{} `json:"data"`
	TraceID   string      `json:"trace_id,omitempty"`
	RequestID string      `json:"request_id,omitempty"`
}

func WriteOK(w http.ResponseWriter, data interface{}) {
	WriteJSON(w, http.StatusOK, string(apperror.CodeOK), "ok", data)
}

func WriteError(w http.ResponseWriter, err error) {
	appErr := apperror.From(err)
	WriteJSON(w, apperror.HTTPStatus(err), string(appErr.Code), appErr.Message, nil)
}

func WriteJSON(w http.ResponseWriter, status int, code string, message string, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	envelope := Envelope{
		Code:    code,
		Message: message,
		Data:    data,
	}
	if status >= http.StatusBadRequest {
		traceContext := traceContextFromResponseHeaders(w)
		observability.InjectTraceHeaders(w, traceContext)
		envelope.TraceID = traceContext.TraceID
		envelope.RequestID = traceContext.RequestID
	}
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(envelope)
}

func GoZeroErrorHandler(err error) (int, any) {
	return GoZeroErrorHandlerCtx(context.Background(), err)
}

func GoZeroErrorHandlerCtx(ctx context.Context, err error) (int, any) {
	appErr := apperror.From(err)
	if appErr == nil {
		return http.StatusOK, Envelope{
			Code:    string(apperror.CodeOK),
			Message: "ok",
			Data:    nil,
		}
	}

	envelope := Envelope{
		Code:    string(appErr.Code),
		Message: appErr.Message,
		Data:    nil,
	}
	if traceContext := observability.TraceContextFromContext(ctx); traceContext.TraceID != "" {
		envelope.TraceID = traceContext.TraceID
		envelope.RequestID = traceContext.RequestID
	}
	return apperror.HTTPStatus(err), envelope
}

func GoZeroUnauthorizedCallback(w http.ResponseWriter, r *http.Request, _ error) {
	_, traceContext := observability.EnsureHTTPTrace(r)
	observability.InjectTraceHeaders(w, traceContext)
	WriteJSON(w, http.StatusUnauthorized, string(apperror.CodeUnauthenticated), "invalid or missing bearer token", nil)
}

func traceContextFromResponseHeaders(w http.ResponseWriter) observability.TraceContext {
	if w == nil {
		return observability.NewTraceContext("", "")
	}
	traceID := strings.TrimSpace(w.Header().Get(observability.HeaderTraceID))
	requestID := strings.TrimSpace(w.Header().Get(observability.HeaderRequestID))
	return observability.NewTraceContext(traceID, requestID)
}
