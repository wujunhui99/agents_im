package llmobs

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	langfuseIngestionPath = "/api/public/ingestion"
	langfuseExportTimeout = 10 * time.Second
)

type LangfuseSink struct {
	config   Config
	endpoint string
	client   *http.Client
}

type langfuseIngestionRequest struct {
	Batch    []langfuseIngestionEvent `json:"batch"`
	Metadata map[string]string        `json:"metadata,omitempty"`
}

type langfuseIngestionEvent struct {
	ID        string         `json:"id"`
	Timestamp string         `json:"timestamp"`
	Type      string         `json:"type"`
	Body      map[string]any `json:"body"`
}

type langfuseIngestionResponse struct {
	Successes []langfuseIngestionSuccess `json:"successes"`
	Errors    []langfuseIngestionError   `json:"errors"`
}

type langfuseIngestionSuccess struct {
	ID     string `json:"id"`
	Status int    `json:"status"`
}

type langfuseIngestionError struct {
	ID      string `json:"id"`
	Status  int    `json:"status"`
	Message string `json:"message"`
}

func newLangfuseSink(config Config) (*LangfuseSink, error) {
	endpoint, err := langfuseEndpoint(config.Langfuse.Host)
	if err != nil {
		return nil, err
	}
	return &LangfuseSink{
		config:   config,
		endpoint: endpoint,
		client:   &http.Client{Timeout: langfuseExportTimeout},
	}, nil
}

func (s *LangfuseSink) Observe(ctx context.Context, event Event) (ObserveResult, error) {
	result := ObserveResult{Backend: BackendLangfuse, Exported: false}
	if s == nil {
		result.DisabledReason = "langfuse sink not configured"
		return result, fmt.Errorf("langfuse sink not configured")
	}
	config := normalizeConfig(s.config)
	if err := validateLangfuseConfig(config.Langfuse); err != nil {
		return result, err
	}
	payload, err := langfusePayload(event, config)
	if err != nil {
		return result, err
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return result, fmt.Errorf("marshal langfuse ingestion payload: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint, bytes.NewReader(body))
	if err != nil {
		return result, fmt.Errorf("build langfuse ingestion request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "agents-im-llmobs/1")
	req.SetBasicAuth(config.Langfuse.PublicKey, config.Langfuse.SecretKey)

	client := s.client
	if client == nil {
		client = &http.Client{Timeout: langfuseExportTimeout}
	}
	resp, err := client.Do(req)
	if err != nil {
		return result, fmt.Errorf("export langfuse ingestion: %w", err)
	}
	defer resp.Body.Close()

	responseBody, readErr := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return result, langfuseHTTPError(resp.StatusCode, responseBody, readErr)
	}
	if readErr != nil {
		return result, fmt.Errorf("read langfuse ingestion response: %w", readErr)
	}
	if err := langfuseIngestionBodyError(responseBody); err != nil {
		return result, err
	}
	result.Exported = true
	return result, nil
}

func langfuseEndpoint(host string) (string, error) {
	host = strings.TrimSpace(host)
	if host == "" {
		return "", ErrLangfuseConfigMissing
	}
	parsed, err := url.Parse(host)
	if err != nil {
		return "", fmt.Errorf("invalid langfuse host: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("invalid langfuse host %q: absolute URL with scheme and host is required", redactURLForError(host))
	}
	basePath := strings.TrimRight(parsed.Path, "/")
	if basePath == "" {
		parsed.Path = langfuseIngestionPath
	} else if strings.HasSuffix(basePath, langfuseIngestionPath) {
		parsed.Path = basePath
	} else {
		parsed.Path = basePath + langfuseIngestionPath
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), nil
}

func langfusePayload(event Event, config Config) (langfuseIngestionRequest, error) {
	event = cloneEvent(event)
	traceID := langfuseTraceID(event)
	occurredAt := langfuseEventTime(event)
	traceEventID := deterministicUUID("langfuse-trace-event", traceID, event.RequestID, event.AgentRunID, event.Type, event.Status)
	observationEventID := deterministicUUID("langfuse-observation-event", traceID, event.RequestID, event.AgentRunID, event.Type, event.Status)

	observationType := "event-create"
	observationBody := langfuseEventObservationBody(traceID, event, config)
	if event.Type == EventTypeGeneration {
		observationType = "generation-create"
		observationBody = langfuseGenerationBody(traceID, event, config)
	}

	return langfuseIngestionRequest{
		Batch: []langfuseIngestionEvent{
			{
				ID:        traceEventID,
				Timestamp: langfuseTimestamp(occurredAt),
				Type:      "trace-create",
				Body:      langfuseTraceBody(traceID, event, config),
			},
			{
				ID:        observationEventID,
				Timestamp: langfuseTimestamp(occurredAt),
				Type:      observationType,
				Body:      observationBody,
			},
		},
		Metadata: map[string]string{"source": "agents_im"},
	}, nil
}

func langfuseTraceBody(traceID string, event Event, config Config) map[string]any {
	body := map[string]any{
		"id":        traceID,
		"timestamp": langfuseTimestamp(langfuseTraceTime(event)),
		"name":      "agents_im.ai_hosting",
		"metadata":  langfuseMetadata(event),
		"tags":      langfuseTags(event),
	}
	if userID := firstNonEmptyString(event.SenderAccountID, event.HostedOwnerAccountID, event.AgentAccountID); userID != "" {
		body["userId"] = userID
	}
	if event.ConversationID != "" {
		body["sessionId"] = event.ConversationID
	}
	if input := langfuseInput(event); len(input) > 0 {
		body["input"] = input
	}
	if output := langfuseOutput(event, config); output != nil {
		body["output"] = output
	}
	return body
}

func langfuseGenerationBody(traceID string, event Event, config Config) map[string]any {
	body := langfuseBaseObservationBody(traceID, event, "agents_im.llm.generation", config)
	if event.ModelName != "" {
		body["model"] = event.ModelName
	}
	if usage := langfuseUsage(event.Generation); len(usage) > 0 {
		body["usage"] = usage
	}
	return body
}

func langfuseEventObservationBody(traceID string, event Event, config Config) map[string]any {
	return langfuseBaseObservationBody(traceID, event, "agents_im.llm.run", config)
}

func langfuseBaseObservationBody(traceID string, event Event, name string, config Config) map[string]any {
	body := map[string]any{
		"id":        langfuseObservationID(traceID, event),
		"traceId":   traceID,
		"name":      name,
		"startTime": langfuseTimestamp(langfuseTraceTime(event)),
		"metadata":  langfuseMetadata(event),
		"level":     langfuseLevel(event),
	}
	if !event.FinishedAt.IsZero() {
		body["endTime"] = langfuseTimestamp(event.FinishedAt)
	}
	if event.ErrorMessage != "" {
		body["statusMessage"] = RedactPlainText(event.ErrorMessage)
	}
	if input := langfuseInput(event); len(input) > 0 {
		body["input"] = input
	}
	if output := langfuseOutput(event, config); output != nil {
		body["output"] = output
	}
	return body
}

func langfuseMetadata(event Event) map[string]any {
	metadata := sanitizeMetadata(event.Metadata)
	setString(metadata, "trace_id", event.TraceID)
	setString(metadata, "request_id", event.RequestID)
	setString(metadata, "agent_run_id", event.AgentRunID)
	setString(metadata, "conversation_id", event.ConversationID)
	setString(metadata, "trigger_server_msg_id", event.TriggerServerMsgID)
	setString(metadata, "response_server_msg_id", event.ResponseServerMsgID)
	setString(metadata, "hosted_owner_account_id", event.HostedOwnerAccountID)
	setString(metadata, "sender_account_id", event.SenderAccountID)
	setString(metadata, "agent_account_id", event.AgentAccountID)
	setString(metadata, "model_provider", event.ModelProvider)
	setString(metadata, "model_name", event.ModelName)
	setString(metadata, "prompt_version", event.PromptVersion)
	setString(metadata, "prompt_hash", event.PromptHash)
	setString(metadata, "runtime_mode", event.RuntimeMode)
	setString(metadata, "event_type", event.Type)
	setString(metadata, "status", event.Status)
	setInt64(metadata, "latency_ms", event.LatencyMs)
	setString(metadata, "error_class", event.ErrorClass)
	if event.ErrorMessage != "" {
		metadata["error_message"] = RedactPlainText(event.ErrorMessage)
	}
	setInt(metadata, "bounded_recent_message_count", event.Generation.BoundedRecentMessageCount)
	if event.Generation.TriggerInContext {
		metadata["trigger_in_context"] = true
	}
	setString(metadata, "finish_reason", event.Generation.FinishReason)
	setInt64(metadata, "prompt_tokens", event.Generation.PromptTokens)
	setInt64(metadata, "completion_tokens", event.Generation.CompletionTokens)
	setInt64(metadata, "reasoning_tokens", event.Generation.ReasoningTokens)
	setInt64(metadata, "cached_tokens", event.Generation.CachedTokens)
	setInt64(metadata, "total_tokens", event.Generation.TotalTokens)
	setInt64(metadata, "generation_latency_ms", event.Generation.LatencyMs)
	return metadata
}

func sanitizeMetadata(input map[string]string) map[string]any {
	out := make(map[string]any, len(input)+24)
	for key, value := range input {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if isSensitiveMetadataKey(key) {
			out[key] = "[REDACTED]"
			continue
		}
		out[key] = RedactPlainText(value)
	}
	return out
}

func isSensitiveMetadataKey(key string) bool {
	normalized := strings.ToLower(strings.TrimSpace(key))
	joined := strings.NewReplacer("-", "_", ".", "_", " ", "_").Replace(normalized)
	if strings.Contains(joined, "apikey") || strings.Contains(joined, "api_key") ||
		strings.Contains(joined, "authorization") || strings.Contains(joined, "cookie") ||
		strings.Contains(joined, "password") || strings.Contains(joined, "passwd") ||
		strings.Contains(joined, "secret") || strings.Contains(joined, "private_key") ||
		strings.Contains(joined, "database_url") || strings.Contains(joined, "connection_string") {
		return true
	}
	for _, token := range strings.FieldsFunc(joined, func(r rune) bool {
		return r < 'a' || r > 'z'
	}) {
		switch token {
		case "token", "bearer", "dsn":
			return true
		}
	}
	return false
}

func langfuseInput(event Event) map[string]any {
	input := make(map[string]any, 4)
	setString(input, "trigger_server_msg_id", event.TriggerServerMsgID)
	setString(input, "prompt_hash", event.PromptHash)
	setInt(input, "bounded_recent_message_count", event.Generation.BoundedRecentMessageCount)
	if event.Generation.TriggerInContext {
		input["trigger_in_context"] = true
	}
	return input
}

func langfuseOutput(event Event, config Config) any {
	if config.CaptureOutput && strings.TrimSpace(event.Generation.FinalOutput) != "" {
		output := captureText(event.Generation.FinalOutput, config.MaxOutputBytes)
		if output == "" {
			return nil
		}
		metadata := map[string]any{
			"captured": true,
			"text":     output,
		}
		if event.Generation.FinalOutputSummary != "" {
			metadata["summary"] = event.Generation.FinalOutputSummary
		}
		return metadata
	}
	if event.Generation.FinalOutputSummary != "" {
		return map[string]any{
			"captured": false,
			"summary":  event.Generation.FinalOutputSummary,
		}
	}
	return nil
}

func langfuseUsage(generation Generation) map[string]any {
	usage := make(map[string]any, 3)
	setInt64(usage, "promptTokens", generation.PromptTokens)
	setInt64(usage, "completionTokens", generation.CompletionTokens)
	setInt64(usage, "totalTokens", generation.TotalTokens)
	return usage
}

func langfuseTags(event Event) []string {
	tags := make([]string, 0, 4)
	for _, tag := range []string{event.RuntimeMode, event.Status, event.ModelProvider, event.Type} {
		tag = strings.TrimSpace(tag)
		if tag != "" {
			tags = append(tags, tag)
		}
	}
	return tags
}

func langfuseLevel(event Event) string {
	if event.Status == StatusFailed || event.ErrorMessage != "" || event.ErrorClass != "" {
		return "ERROR"
	}
	return "DEFAULT"
}

func langfuseTraceID(event Event) string {
	if value := firstNonEmptyString(event.TraceID, event.RequestID, event.AgentRunID); value != "" {
		return value
	}
	return deterministicUUID("langfuse-trace", event.ConversationID, event.TriggerServerMsgID, event.ResponseServerMsgID, event.Type, event.Status)
}

func langfuseObservationID(traceID string, event Event) string {
	if event.AgentRunID != "" {
		return strings.Join([]string{event.AgentRunID, event.Type, event.Status}, ":")
	}
	return deterministicUUID("langfuse-observation", traceID, event.RequestID, event.ConversationID, event.TriggerServerMsgID, event.Type, event.Status)
}

func langfuseEventTime(event Event) time.Time {
	if !event.FinishedAt.IsZero() {
		return event.FinishedAt.UTC()
	}
	if !event.StartedAt.IsZero() {
		return event.StartedAt.UTC()
	}
	return time.Now().UTC()
}

func langfuseTraceTime(event Event) time.Time {
	if !event.StartedAt.IsZero() {
		return event.StartedAt.UTC()
	}
	return langfuseEventTime(event)
}

func langfuseTimestamp(t time.Time) string {
	if t.IsZero() {
		t = time.Now().UTC()
	}
	return t.UTC().Format(time.RFC3339Nano)
}

func langfuseIngestionBodyError(responseBody []byte) error {
	responseBody = bytes.TrimSpace(responseBody)
	if len(responseBody) == 0 {
		return nil
	}
	var response langfuseIngestionResponse
	if err := json.Unmarshal(responseBody, &response); err != nil {
		return fmt.Errorf("decode langfuse ingestion response: %w", err)
	}
	if len(response.Errors) == 0 {
		return nil
	}
	firstError := response.Errors[0]
	message := RedactPlainText(firstError.Message)
	if message == "" {
		message = fmt.Sprintf("status=%d", firstError.Status)
	}
	if firstError.ID != "" {
		return fmt.Errorf("langfuse ingestion rejected event id=%q status=%d: %s", firstError.ID, firstError.Status, message)
	}
	return fmt.Errorf("langfuse ingestion rejected event status=%d: %s", firstError.Status, message)
}

func langfuseHTTPError(status int, responseBody []byte, readErr error) error {
	if readErr != nil {
		return fmt.Errorf("langfuse export failed: status=%d; read response: %w", status, readErr)
	}
	message := RedactPlainText(string(responseBody))
	if message == "" {
		return fmt.Errorf("langfuse export failed: status=%d", status)
	}
	return fmt.Errorf("langfuse export failed: status=%d body=%q", status, message)
}

func deterministicUUID(parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	bytes := sum[:16]
	bytes[6] = (bytes[6] & 0x0f) | 0x50
	bytes[8] = (bytes[8] & 0x3f) | 0x80
	hexValue := hex.EncodeToString(bytes)
	return fmt.Sprintf("%s-%s-%s-%s-%s", hexValue[0:8], hexValue[8:12], hexValue[12:16], hexValue[16:20], hexValue[20:32])
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func setString(metadata map[string]any, key string, value string) {
	value = strings.TrimSpace(value)
	if value != "" {
		metadata[key] = RedactPlainText(value)
	}
}

func setInt(metadata map[string]any, key string, value int) {
	if value != 0 {
		metadata[key] = value
	}
}

func setInt64(metadata map[string]any, key string, value int64) {
	if value != 0 {
		metadata[key] = value
	}
}

func redactURLForError(value string) string {
	parsed, err := url.Parse(value)
	if err != nil || parsed == nil {
		return RedactPlainText(value)
	}
	parsed.RawQuery = ""
	parsed.User = nil
	return parsed.String()
}
