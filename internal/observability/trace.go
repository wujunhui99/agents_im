package observability

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	HeaderTraceID     = "X-Trace-Id"
	HeaderRequestID   = "X-Request-Id"
	HeaderTraceparent = "Traceparent"
)

type traceContextKey string

const (
	traceIDContextKey   traceContextKey = "trace_id"
	requestIDContextKey traceContextKey = "request_id"
)

type TraceContext struct {
	TraceID   string
	RequestID string
}

func TraceMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		tracedRequest, traceContext := EnsureHTTPTrace(r)
		recorder := &statusRecorder{ResponseWriter: w}
		InjectTraceHeaders(recorder, traceContext)

		next.ServeHTTP(recorder, tracedRequest)

		status := recorder.status
		if status == 0 {
			status = http.StatusOK
		}
		RecordHTTPRequest(tracedRequest.Method, tracedRequest.URL.Path, status)
		log.Printf(
			"http_request trace_id=%s request_id=%s method=%s path=%s status=%d duration_ms=%d remote_addr=%s user_agent=%q",
			traceContext.TraceID,
			traceContext.RequestID,
			tracedRequest.Method,
			tracedRequest.URL.Path,
			status,
			time.Since(start).Milliseconds(),
			tracedRequest.RemoteAddr,
			tracedRequest.UserAgent(),
		)
	})
}

func TraceMiddlewareFunc(next http.HandlerFunc) http.HandlerFunc {
	return TraceMiddleware(next).ServeHTTP
}

func EnsureHTTPTrace(r *http.Request) (*http.Request, TraceContext) {
	if r == nil {
		return r, NewTraceContext("", "")
	}
	if existing := TraceContextFromContext(r.Context()); existing.TraceID != "" {
		return r, existing
	}
	traceContext := TraceContextFromHTTPRequest(r)
	return r.WithContext(ContextWithTrace(r.Context(), traceContext)), traceContext
}

func TraceContextFromHTTPRequest(r *http.Request) TraceContext {
	if r == nil {
		return NewTraceContext("", "")
	}
	traceID := strings.TrimSpace(r.Header.Get(HeaderTraceID))
	if traceID == "" {
		traceID = traceIDFromTraceparent(r.Header.Get(HeaderTraceparent))
	}
	requestID := strings.TrimSpace(r.Header.Get(HeaderRequestID))
	return NewTraceContext(traceID, requestID)
}

func NewTraceContext(traceID string, requestID string) TraceContext {
	traceID = sanitizeID(traceID)
	if traceID == "" {
		traceID = newRandomID("trace")
	}
	requestID = sanitizeID(requestID)
	if requestID == "" {
		requestID = traceID
	}
	return TraceContext{TraceID: traceID, RequestID: requestID}
}

func ContextWithTrace(ctx context.Context, traceContext TraceContext) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	traceContext = NewTraceContext(traceContext.TraceID, traceContext.RequestID)
	ctx = context.WithValue(ctx, traceIDContextKey, traceContext.TraceID)
	ctx = context.WithValue(ctx, requestIDContextKey, traceContext.RequestID)
	return ctx
}

func TraceContextFromContext(ctx context.Context) TraceContext {
	if ctx == nil {
		return TraceContext{}
	}
	traceID, _ := ctx.Value(traceIDContextKey).(string)
	requestID, _ := ctx.Value(requestIDContextKey).(string)
	if strings.TrimSpace(traceID) == "" {
		return TraceContext{}
	}
	return TraceContext{
		TraceID:   strings.TrimSpace(traceID),
		RequestID: strings.TrimSpace(requestID),
	}
}

func TraceIDFromContext(ctx context.Context) string {
	return TraceContextFromContext(ctx).TraceID
}

func RequestIDFromContext(ctx context.Context) string {
	return TraceContextFromContext(ctx).RequestID
}

func InjectTraceHeaders(w http.ResponseWriter, traceContext TraceContext) {
	if w == nil {
		return
	}
	traceContext = NewTraceContext(traceContext.TraceID, traceContext.RequestID)
	w.Header().Set(HeaderTraceID, traceContext.TraceID)
	w.Header().Set(HeaderRequestID, traceContext.RequestID)
}

func traceIDFromTraceparent(value string) string {
	parts := strings.Split(strings.TrimSpace(value), "-")
	if len(parts) < 4 || len(parts[1]) != 32 {
		return ""
	}
	if _, err := hex.DecodeString(parts[1]); err != nil {
		return ""
	}
	return parts[1]
}

func sanitizeID(value string) string {
	value = strings.TrimSpace(value)
	if len(value) < 8 || len(value) > 128 {
		return ""
	}
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '-' || r == '_' || r == '.':
		default:
			return ""
		}
	}
	return value
}

func newRandomID(prefix string) string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err == nil {
		return prefix + "_" + hex.EncodeToString(b[:])
	}
	return prefix + "_" + strconv.FormatInt(time.Now().UTC().UnixNano(), 36)
}

type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int64
}

func (r *statusRecorder) WriteHeader(status int) {
	if r.status != 0 {
		return
	}
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Write(data []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	n, err := r.ResponseWriter.Write(data)
	r.bytes += int64(n)
	return n, err
}

func (r *statusRecorder) Flush() {
	if flusher, ok := r.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (r *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := r.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("response writer does not support hijacking")
	}
	if r.status == 0 {
		r.status = http.StatusSwitchingProtocols
	}
	return hijacker.Hijack()
}

func (r *statusRecorder) Push(target string, opts *http.PushOptions) error {
	pusher, ok := r.ResponseWriter.(http.Pusher)
	if !ok {
		return http.ErrNotSupported
	}
	return pusher.Push(target, opts)
}

func (r *statusRecorder) ReadFrom(reader io.Reader) (int64, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	if readerFrom, ok := r.ResponseWriter.(io.ReaderFrom); ok {
		n, err := readerFrom.ReadFrom(reader)
		r.bytes += n
		return n, err
	}
	return io.Copy(r.ResponseWriter, reader)
}
