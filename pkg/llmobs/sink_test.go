package llmobs

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewSinkMissingLangfuseConfigIsExplicitlyDisabled(t *testing.T) {
	sink, err := NewSink(Config{
		Enabled: true,
		Backend: BackendLangfuse,
		Langfuse: LangfuseConfig{
			Host: "https://langfuse.agenticim.xyz",
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

func TestLangfuseSinkExportsIngestionBatchWithBasicAuth(t *testing.T) {
	var captured map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/api/public/ingestion" {
			t.Fatalf("path = %s, want /api/public/ingestion", r.URL.Path)
		}
		publicKey, secretKey, ok := r.BasicAuth()
		if !ok || publicKey != "pk-lf-unit-test" || secretKey != "sk-lf-unit-test" {
			t.Fatalf("unexpected basic auth public=%q secret_ok=%v", publicKey, secretKey == "sk-lf-unit-test")
		}
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMultiStatus)
		_, _ = w.Write([]byte(`{"successes":[{"id":"accepted","status":201}],"errors":[]}`))
	}))
	defer server.Close()

	sink, err := NewSink(Config{
		Enabled:       true,
		Backend:       BackendLangfuse,
		CaptureOutput: true,
		Langfuse: LangfuseConfig{
			Host:      server.URL + "/",
			PublicKey: "pk-lf-unit-test",
			SecretKey: "sk-lf-unit-test",
		},
	})
	if err != nil {
		t.Fatalf("new langfuse sink: %v", err)
	}
	if sink != nil {
		result, err := sink.Observe(context.Background(), sampleLangfuseEvent())
		if err != nil {
			t.Fatalf("observe langfuse event: %v", err)
		}
		if result.Backend != BackendLangfuse || !result.Exported {
			t.Fatalf("observe result = %+v, want langfuse exported", result)
		}
	}
	if captured == nil {
		t.Fatalf("langfuse server did not capture a request")
	}
	batch, ok := captured["batch"].([]any)
	if !ok || len(batch) != 2 {
		t.Fatalf("batch = %#v, want trace plus observation events", captured["batch"])
	}
	types := map[string]bool{}
	for _, item := range batch {
		event, ok := item.(map[string]any)
		if !ok {
			t.Fatalf("batch item = %#v, want object", item)
		}
		if strings.TrimSpace(event["id"].(string)) == "" || strings.TrimSpace(event["timestamp"].(string)) == "" {
			t.Fatalf("event missing envelope id/timestamp: %#v", event)
		}
		types[event["type"].(string)] = true
	}
	if !types["trace-create"] || !types["generation-create"] {
		t.Fatalf("event types = %+v, want trace-create and generation-create", types)
	}
	payload, err := json.Marshal(captured)
	if err != nil {
		t.Fatalf("marshal captured payload: %v", err)
	}
	if !strings.Contains(string(payload), "visible generated answer") {
		t.Fatalf("capture output was enabled, payload should include final output: %s", payload)
	}
}

func TestLangfuseSinkNon2xxReturnsExplicitError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "upstream unavailable", http.StatusBadGateway)
	}))
	defer server.Close()

	sink, err := NewSink(Config{
		Enabled: true,
		Backend: BackendLangfuse,
		Langfuse: LangfuseConfig{
			Host:      server.URL,
			PublicKey: "pk-lf-unit-test",
			SecretKey: "sk-lf-unit-test",
		},
	})
	if err != nil {
		t.Fatalf("new langfuse sink: %v", err)
	}
	result, err := sink.Observe(context.Background(), sampleLangfuseEvent())
	if err == nil {
		t.Fatalf("expected explicit export error")
	}
	if result.Exported {
		t.Fatalf("failed langfuse export must not report exported=true: %+v", result)
	}
	if !strings.Contains(err.Error(), "status=502") {
		t.Fatalf("error = %v, want status detail", err)
	}
	if strings.Contains(err.Error(), "sk-lf-unit-test") {
		t.Fatalf("error leaked secret key: %v", err)
	}
}

func TestLangfuseSinkIngestionErrorsReturnExplicitError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMultiStatus)
		_, _ = w.Write([]byte(`{"successes":[],"errors":[{"id":"bad-event","status":400,"message":"invalid payload"}]}`))
	}))
	defer server.Close()

	sink, err := NewSink(Config{
		Enabled: true,
		Backend: BackendLangfuse,
		Langfuse: LangfuseConfig{
			Host:      server.URL,
			PublicKey: "pk-lf-unit-test",
			SecretKey: "sk-lf-unit-test",
		},
	})
	if err != nil {
		t.Fatalf("new langfuse sink: %v", err)
	}
	result, err := sink.Observe(context.Background(), sampleLangfuseEvent())
	if err == nil {
		t.Fatalf("expected explicit ingestion error")
	}
	if result.Exported {
		t.Fatalf("ingestion error must not report exported=true: %+v", result)
	}
	if !strings.Contains(err.Error(), "invalid payload") {
		t.Fatalf("error = %v, want ingestion error detail", err)
	}
}

func TestLangfuseSinkRedactsSensitiveMetadataAndSuppressesOutputByDefault(t *testing.T) {
	var capturedBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		raw, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("marshal payload: %v", err)
		}
		capturedBody = string(raw)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"successes":[{"id":"accepted","status":201}],"errors":[]}`))
	}))
	defer server.Close()

	sink, err := NewSink(Config{
		Enabled:       true,
		Backend:       BackendLangfuse,
		CaptureOutput: false,
		Langfuse: LangfuseConfig{
			Host:      server.URL,
			PublicKey: "pk-lf-unit-test",
			SecretKey: "sk-lf-unit-test",
		},
	})
	if err != nil {
		t.Fatalf("new langfuse sink: %v", err)
	}
	event := sampleLangfuseEvent()
	event.Generation.FinalOutput = "raw final output that should stay local"
	event.Generation.FinalOutputSummary = TextSummary(event.Generation.FinalOutput)
	event.ErrorMessage = "provider failed bearer unit-sensitive-bearer token=unit-sensitive-token password=unit-sensitive-password dsn=postgres://unit:unit@db/agents"
	event.Metadata = map[string]string{
		"api_key":       "unit-sensitive-api-key",
		"authorization": "Bearer unit-sensitive-bearer",
		"cookie":        "session=unit-sensitive-cookie",
		"database_dsn":  "postgres://unit:unit@db/agents",
		"normal":        "safe metadata",
		"password":      "unit-sensitive-password",
	}
	result, err := sink.Observe(context.Background(), event)
	if err != nil {
		t.Fatalf("observe langfuse event: %v", err)
	}
	if !result.Exported {
		t.Fatalf("observe result = %+v, want exported", result)
	}
	for _, forbidden := range []string{
		"raw final output that should stay local",
		"unit-sensitive-api-key",
		"unit-sensitive-bearer",
		"unit-sensitive-cookie",
		"unit-sensitive-password",
		"postgres://unit:unit@db/agents",
	} {
		if strings.Contains(capturedBody, forbidden) {
			t.Fatalf("langfuse payload leaked %q: %s", forbidden, capturedBody)
		}
	}
	if !strings.Contains(capturedBody, "[REDACTED]") {
		t.Fatalf("langfuse payload should include redaction markers: %s", capturedBody)
	}
	if !strings.Contains(capturedBody, event.Generation.FinalOutputSummary) {
		t.Fatalf("langfuse payload should keep output summary for correlation: %s", capturedBody)
	}
}

func TestRedactPlainTextRedactsInlineSecrets(t *testing.T) {
	input := strings.Join([]string{
		"bearer unit-sensitive-bearer",
		"token=unit-sensitive-token",
		"cookie=unit-sensitive-cookie",
		"password=unit-sensitive-password",
		"postgres://unit:unit@db/agents",
		"eyJunit.header.eyJunit.payload.unit_signature",
		"sk-unitsecret000000",
	}, " ")
	got := RedactPlainText(input)
	for _, forbidden := range []string{
		"unit-sensitive-bearer",
		"unit-sensitive-token",
		"unit-sensitive-cookie",
		"unit-sensitive-password",
		"postgres://unit:unit@db/agents",
		"eyJunit.header.eyJunit.payload.unit_signature",
		"sk-unitsecret000000",
	} {
		if strings.Contains(got, forbidden) {
			t.Fatalf("redacted text leaked %q: %s", forbidden, got)
		}
	}
	if !strings.Contains(got, "[REDACTED]") {
		t.Fatalf("redacted text should contain marker: %s", got)
	}
}

func sampleLangfuseEvent() Event {
	return Event{
		Type:                 EventTypeGeneration,
		Status:               StatusSucceeded,
		TraceID:              "trace_langfuse_1",
		RequestID:            "req_langfuse_1",
		AgentRunID:           "run_langfuse_1",
		ConversationID:       "conv_langfuse_1",
		TriggerServerMsgID:   "msg_trigger_1",
		ResponseServerMsgID:  "msg_response_1",
		HostedOwnerAccountID: "owner_1",
		SenderAccountID:      "sender_1",
		AgentAccountID:       "agent_1",
		ModelProvider:        "deepseek",
		ModelName:            "deepseek-unit",
		PromptVersion:        "v1",
		PromptHash:           "prompt_hash_1",
		RuntimeMode:          RuntimeModeAIHostingAutoReply,
		LatencyMs:            42,
		Generation: Generation{
			BoundedRecentMessageCount: 4,
			TriggerInContext:          true,
			FinishReason:              "stop",
			PromptTokens:              10,
			CompletionTokens:          5,
			ReasoningTokens:           2,
			CachedTokens:              1,
			TotalTokens:               17,
			LatencyMs:                 42,
			FinalOutput:               "visible generated answer",
			FinalOutputSummary:        TextSummary("visible generated answer"),
		},
		StartedAt:  time.Date(2026, 5, 21, 10, 0, 0, 0, time.UTC),
		FinishedAt: time.Date(2026, 5, 21, 10, 0, 1, 0, time.UTC),
		Metadata: map[string]string{
			"safe_key": "safe value",
		},
	}
}
