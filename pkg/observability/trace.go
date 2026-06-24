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
	"net/url"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	oteltrace "go.opentelemetry.io/otel/trace"
)

const (
	HeaderTraceID     = "X-Trace-Id"
	HeaderRequestID   = "X-Request-Id"
	HeaderTraceparent = "traceparent"
	HeaderTracestate  = "tracestate"
)

type traceContextKey string

const (
	traceIDContextKey   traceContextKey = "trace_id"
	requestIDContextKey traceContextKey = "request_id"
	traceParentKey      traceContextKey = "traceparent"
	traceStateKey       traceContextKey = "tracestate"
)

type TraceContext struct {
	TraceID     string
	RequestID   string
	TraceParent string
	TraceState  string
}

func TraceMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		tracedRequest, traceContext := EnsureHTTPTrace(r)
		route := RouteTemplate(tracedRequest.URL.Path)
		spanName := tracedRequest.Method + " " + route
		var span oteltrace.Span
		if !IsNoisyRoute(route) {
			tracedCtx, startedSpan := StartSpan(
				tracedRequest.Context(),
				spanName,
				oteltrace.WithSpanKind(oteltrace.SpanKindServer),
				oteltrace.WithAttributes(
					attribute.String("http.request.method", tracedRequest.Method),
					attribute.String("url.path", tracedRequest.URL.Path),
					attribute.String("http.route", route),
				),
			)
			span = startedSpan
			traceContext = TraceContextFromContext(tracedCtx)
			tracedRequest = tracedRequest.WithContext(tracedCtx)
		}
		recorder := &statusRecorder{ResponseWriter: w}
		InjectTraceHeaders(recorder, traceContext)

		next.ServeHTTP(recorder, tracedRequest)

		status := recorder.status
		if status == 0 {
			status = http.StatusOK
		}
		if span != nil {
			span.SetAttributes(attribute.Int("http.response.status_code", status))
			if status >= http.StatusInternalServerError {
				span.SetStatus(codes.Error, http.StatusText(status))
			}
			span.End()
		}
		RecordHTTPRequest(tracedRequest.Method, tracedRequest.URL.Path, status)
		if IsNoisyRoute(route) {
			return
		}
		log.Printf(
			"http_request trace_id=%s request_id=%s method=%s route=%s path=%s status=%d duration_ms=%d remote_addr=%s user_agent=%q",
			traceContext.TraceID,
			traceContext.RequestID,
			tracedRequest.Method,
			route,
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
	ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))
	traceContext := TraceContextFromHTTPRequest(r)
	if spanContext := oteltrace.SpanContextFromContext(ctx); spanContext.IsValid() {
		traceContext.TraceID = spanContext.TraceID().String()
		if traceContext.TraceParent == "" {
			traceContext.TraceParent = traceparentFromSpanContext(spanContext)
		}
		if traceContext.TraceState == "" {
			traceContext.TraceState = spanContext.TraceState().String()
		}
	} else if legacyParent := remoteSpanContextFromTrace(traceContext); legacyParent.IsValid() {
		ctx = oteltrace.ContextWithRemoteSpanContext(ctx, legacyParent)
	}
	ctx = ContextWithTrace(ctx, traceContext)
	return r.WithContext(ctx), TraceContextFromContext(ctx)
}

func TraceContextFromHTTPRequest(r *http.Request) TraceContext {
	if r == nil {
		return NewTraceContext("", "")
	}
	traceID := strings.TrimSpace(r.Header.Get(HeaderTraceID))
	traceParent := strings.TrimSpace(r.Header.Get(HeaderTraceparent))
	traceState := strings.TrimSpace(r.Header.Get(HeaderTracestate))
	if traceParentID := traceIDFromTraceparent(traceParent); traceParentID != "" {
		traceID = traceParentID
	}
	requestID := strings.TrimSpace(r.Header.Get(HeaderRequestID))
	traceContext := NewTraceContext(traceID, requestID)
	traceContext.TraceParent = sanitizeTraceparent(traceParent)
	traceContext.TraceState = sanitizeTracestate(traceState)
	return traceContext
}

func NewTraceContext(traceID string, requestID string) TraceContext {
	traceID = sanitizeID(traceID)
	if traceID == "" {
		traceID = newTraceID()
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
	traceParent := traceContext.TraceParent
	traceState := traceContext.TraceState
	traceContext = NewTraceContext(traceContext.TraceID, traceContext.RequestID)
	traceContext.TraceParent = sanitizeTraceparent(traceParent)
	traceContext.TraceState = sanitizeTracestate(traceState)
	if traceContext.TraceParent == "" && isCanonicalTraceID(traceContext.TraceID) {
		traceContext.TraceParent = traceparentFromTraceID(traceContext.TraceID)
	}
	ctx = context.WithValue(ctx, traceIDContextKey, traceContext.TraceID)
	ctx = context.WithValue(ctx, requestIDContextKey, traceContext.RequestID)
	ctx = context.WithValue(ctx, traceParentKey, traceContext.TraceParent)
	ctx = context.WithValue(ctx, traceStateKey, traceContext.TraceState)
	return ctx
}

func TraceContextFromContext(ctx context.Context) TraceContext {
	if ctx == nil {
		return TraceContext{}
	}
	if spanContext := oteltrace.SpanContextFromContext(ctx); spanContext.IsValid() {
		requestID, _ := ctx.Value(requestIDContextKey).(string)
		if strings.TrimSpace(requestID) == "" {
			requestID = spanContext.TraceID().String()
		}
		return TraceContext{
			TraceID:     spanContext.TraceID().String(),
			RequestID:   strings.TrimSpace(requestID),
			TraceParent: traceparentFromSpanContext(spanContext),
			TraceState:  spanContext.TraceState().String(),
		}
	}
	traceID, _ := ctx.Value(traceIDContextKey).(string)
	requestID, _ := ctx.Value(requestIDContextKey).(string)
	if strings.TrimSpace(traceID) == "" {
		return TraceContext{}
	}
	traceParent, _ := ctx.Value(traceParentKey).(string)
	traceState, _ := ctx.Value(traceStateKey).(string)
	return TraceContext{
		TraceID:     strings.TrimSpace(traceID),
		RequestID:   strings.TrimSpace(requestID),
		TraceParent: strings.TrimSpace(traceParent),
		TraceState:  strings.TrimSpace(traceState),
	}
}

func TraceIDFromContext(ctx context.Context) string {
	return TraceContextFromContext(ctx).TraceID
}

func InjectTraceHeaders(w http.ResponseWriter, traceContext TraceContext) {
	if w == nil {
		return
	}
	traceParent := traceContext.TraceParent
	traceState := traceContext.TraceState
	traceContext = NewTraceContext(traceContext.TraceID, traceContext.RequestID)
	traceContext.TraceParent = sanitizeTraceparent(traceParent)
	traceContext.TraceState = sanitizeTracestate(traceState)
	w.Header().Set(HeaderTraceID, traceContext.TraceID)
	w.Header().Set(HeaderRequestID, traceContext.RequestID)
	if traceContext.TraceParent != "" {
		w.Header().Set(HeaderTraceparent, traceContext.TraceParent)
	}
	if traceContext.TraceState != "" {
		w.Header().Set(HeaderTracestate, traceContext.TraceState)
	}
}

func ExtractTraceContext(ctx context.Context, carrier propagation.TextMapCarrier) TraceContext {
	if ctx == nil {
		ctx = context.Background()
	}
	if carrier == nil {
		return TraceContextFromContext(ctx)
	}
	traceParent := strings.TrimSpace(carrier.Get(HeaderTraceparent))
	traceState := strings.TrimSpace(carrier.Get(HeaderTracestate))
	traceID := strings.TrimSpace(carrier.Get(HeaderTraceID))
	if traceParentID := traceIDFromTraceparent(traceParent); traceParentID != "" {
		traceID = traceParentID
	}
	requestID := strings.TrimSpace(carrier.Get(HeaderRequestID))
	traceContext := NewTraceContext(traceID, requestID)
	traceContext.TraceParent = sanitizeTraceparent(traceParent)
	traceContext.TraceState = sanitizeTracestate(traceState)
	return traceContext
}

func InjectTraceContext(ctx context.Context, carrier propagation.TextMapCarrier) {
	if carrier == nil {
		return
	}
	traceContext := TraceContextFromContext(ctx)
	if traceContext.TraceID == "" {
		traceContext = NewTraceContext("", "")
	}
	carrier.Set(HeaderTraceID, traceContext.TraceID)
	carrier.Set(HeaderRequestID, traceContext.RequestID)
	if traceContext.TraceParent != "" {
		carrier.Set(HeaderTraceparent, traceContext.TraceParent)
	}
	if traceContext.TraceState != "" {
		carrier.Set(HeaderTracestate, traceContext.TraceState)
	}
}

func StartSpan(ctx context.Context, name string, opts ...oteltrace.SpanStartOption) (context.Context, oteltrace.Span) {
	if ctx == nil {
		ctx = context.Background()
	}
	if tc := TraceContextFromContext(ctx); tc.TraceID != "" {
		if parent := remoteSpanContextFromTrace(tc); parent.IsValid() && !oteltrace.SpanContextFromContext(ctx).IsValid() {
			ctx = oteltrace.ContextWithRemoteSpanContext(ctx, parent)
		}
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = "operation"
	}
	ctx, span := otel.Tracer("github.com/wujunhui99/agents_im/pkg/observability").Start(ctx, name, opts...)
	if sc := span.SpanContext(); sc.IsValid() {
		tc := TraceContextFromContext(ctx)
		ctx = ContextWithTrace(ctx, tc)
	}
	return ctx, span
}

func RecordSpanError(span oteltrace.Span, err error) {
	if span == nil || err == nil {
		return
	}
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
}

func RouteTemplate(path string) string {
	parsed, err := url.Parse(strings.TrimSpace(path))
	if err == nil && parsed.Path != "" {
		path = parsed.Path
	}
	path = strings.TrimSpace(path)
	if path == "" || path == "/" {
		return "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	switch path {
	case "/healthz", "/readyz", "/metrics", "/ws":
		return path
	}
	parts := strings.Split(strings.Trim(path, "/"), "/")
	for i, part := range parts {
		if i > 0 && parts[i-1] == "llm-traces" {
			parts[i] = ":trace_id"
			continue
		}
		if isDynamicRouteSegment(part) {
			parts[i] = ":id"
		}
	}
	return "/" + strings.Join(parts, "/")
}

func IsNoisyRoute(route string) bool {
	switch strings.TrimSpace(route) {
	case "/healthz", "/readyz", "/metrics":
		return true
	default:
		return false
	}
}

func TraceUIURL(baseURL string, traceID string) string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	traceID = strings.ToLower(strings.TrimSpace(traceID))
	if baseURL == "" || !isCanonicalTraceID(traceID) {
		return ""
	}
	left := fmt.Sprintf(`{"datasource":"Tempo","queries":[{"queryType":"traceId","query":"%s"}],"range":{"from":"now-6h","to":"now"}}`, traceID)
	return baseURL + "/explore?left=" + url.QueryEscape(left)
}

func traceIDFromTraceparent(value string) string {
	parts := strings.Split(strings.TrimSpace(value), "-")
	if len(parts) < 4 || len(parts[1]) != 32 {
		return ""
	}
	traceID := strings.ToLower(parts[1])
	if !isCanonicalTraceID(traceID) {
		return ""
	}
	return traceID
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

func newTraceID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err == nil {
		traceID := hex.EncodeToString(b[:])
		if isCanonicalTraceID(traceID) {
			return traceID
		}
	}
	return fmt.Sprintf("%032x", time.Now().UTC().UnixNano())
}

func newSpanIDHex() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err == nil {
		spanID := hex.EncodeToString(b[:])
		if spanID != "0000000000000000" {
			return spanID
		}
	}
	return fmt.Sprintf("%016x", time.Now().UTC().UnixNano())
}

func isCanonicalTraceID(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	if len(value) != 32 || value == "00000000000000000000000000000000" {
		return false
	}
	if _, err := hex.DecodeString(value); err != nil {
		return false
	}
	return true
}

func traceparentFromTraceID(traceID string) string {
	traceID = strings.ToLower(strings.TrimSpace(traceID))
	if !isCanonicalTraceID(traceID) {
		return ""
	}
	return "00-" + traceID + "-" + newSpanIDHex() + "-01"
}

func traceparentFromSpanContext(spanContext oteltrace.SpanContext) string {
	if !spanContext.IsValid() {
		return ""
	}
	flags := "00"
	if spanContext.TraceFlags().IsSampled() {
		flags = "01"
	}
	return "00-" + spanContext.TraceID().String() + "-" + spanContext.SpanID().String() + "-" + flags
}

func sanitizeTraceparent(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	parts := strings.Split(value, "-")
	if len(parts) != 4 || parts[0] != "00" || !isCanonicalTraceID(parts[1]) || len(parts[2]) != 16 || len(parts[3]) != 2 {
		return ""
	}
	if parts[2] == "0000000000000000" {
		return ""
	}
	if _, err := hex.DecodeString(parts[2]); err != nil {
		return ""
	}
	if _, err := hex.DecodeString(parts[3]); err != nil {
		return ""
	}
	return value
}

func sanitizeTracestate(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > 512 {
		return ""
	}
	for _, r := range value {
		if r < 0x20 || r > 0x7e {
			return ""
		}
	}
	return value
}

func remoteSpanContextFromTrace(traceContext TraceContext) oteltrace.SpanContext {
	traceID, err := oteltrace.TraceIDFromHex(strings.ToLower(strings.TrimSpace(traceContext.TraceID)))
	if err != nil || !traceID.IsValid() {
		return oteltrace.SpanContext{}
	}
	spanIDHex := ""
	if parent := sanitizeTraceparent(traceContext.TraceParent); parent != "" {
		parts := strings.Split(parent, "-")
		spanIDHex = parts[2]
	}
	if spanIDHex == "" {
		spanIDHex = newSpanIDHex()
	}
	spanID, err := oteltrace.SpanIDFromHex(spanIDHex)
	if err != nil || !spanID.IsValid() {
		return oteltrace.SpanContext{}
	}
	config := oteltrace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: oteltrace.FlagsSampled,
		Remote:     true,
	}
	if traceContext.TraceState != "" {
		if traceState, err := oteltrace.ParseTraceState(traceContext.TraceState); err == nil {
			config.TraceState = traceState
		}
	}
	return oteltrace.NewSpanContext(config)
}

func isDynamicRouteSegment(part string) bool {
	part = strings.TrimSpace(part)
	if part == "" {
		return false
	}
	if len(part) >= 24 && isMostlyHexOrID(part) {
		return true
	}
	if len(part) >= 8 && strings.Contains(part, "_") && isMostlyHexOrID(part) {
		return true
	}
	for _, r := range part {
		if r < '0' || r > '9' {
			return false
		}
	}
	return len(part) > 0
}

func isMostlyHexOrID(value string) bool {
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '-' || r == '_':
		default:
			return false
		}
	}
	return true
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
