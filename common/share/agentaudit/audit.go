package agentaudit

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/wujunhui99/agents_im/pkg/apperror"
)

type Status string

const (
	StatusStarted   Status = "started"
	StatusSucceeded Status = "succeeded"
	StatusFailed    Status = "failed"
	StatusCancelled Status = "cancelled"

	RedactedValue = "[REDACTED]"
)

const (
	maxAuditTextLength = 1024
)

var inlineSecretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(bearer\s+)[a-z0-9._~+/=-]+`),
	regexp.MustCompile(`(?i)((?:api[_-]?key|access[_-]?token|refresh[_-]?token|token|password|secret)=)[^\s&]+`),
}

type Summary map[string]any

func (s Summary) Clone() Summary {
	cloned, err := RedactSummary(s)
	if err != nil {
		return Summary{}
	}
	return cloned
}

func (s Summary) String() string {
	if s == nil {
		return "{}"
	}
	encoded, err := json.Marshal(s)
	if err != nil {
		return "{}"
	}
	return string(encoded)
}

type AgentRun struct {
	RunID            string
	AgentID          string
	ConversationID   string
	TriggerMessageID string
	RequestingUserID string
	Status           Status
	InputSummary     Summary
	OutputSummary    Summary
	OutputMessageID  string
	ErrorCode        string
	ErrorMessage     string
	TraceID          string
	RequestID        string
	StartedAt        time.Time
	FinishedAt       time.Time
	CreatedAt        time.Time
}

func (r AgentRun) Clone() AgentRun {
	r.InputSummary = r.InputSummary.Clone()
	r.OutputSummary = r.OutputSummary.Clone()
	return r
}

type AgentToolCall struct {
	ToolCallID    string
	RunID         string
	AgentID       string
	ToolID        string
	ToolName      string
	Status        Status
	InputSummary  Summary
	OutputSummary Summary
	DurationMs    int64
	ErrorCode     string
	ErrorMessage  string
	TraceID       string
	RequestID     string
	StartedAt     time.Time
	FinishedAt    time.Time
	CreatedAt     time.Time
}

func (c AgentToolCall) Clone() AgentToolCall {
	c.InputSummary = c.InputSummary.Clone()
	c.OutputSummary = c.OutputSummary.Clone()
	return c
}

type AgentFileRead struct {
	FileReadID     string
	RunID          string
	AgentID        string
	SkillID        string
	FileID         string
	ObjectKey      string
	SHA256         string
	Status         Status
	ByteCount      int64
	ContentSummary Summary
	ErrorCode      string
	ErrorMessage   string
	TraceID        string
	RequestID      string
	StartedAt      time.Time
	FinishedAt     time.Time
	CreatedAt      time.Time
}

func (r AgentFileRead) Clone() AgentFileRead {
	r.ContentSummary = r.ContentSummary.Clone()
	return r
}

type AgentPythonExec struct {
	PythonExecID     string
	RunID            string
	AgentID          string
	SandboxRequestID string
	Status           Status
	CodeSummary      Summary
	ResourceSummary  Summary
	StdoutSummary    Summary
	StderrSummary    Summary
	ResultSummary    Summary
	ErrorCode        string
	ErrorMessage     string
	TraceID          string
	RequestID        string
	StartedAt        time.Time
	FinishedAt       time.Time
	CreatedAt        time.Time
}

func (e AgentPythonExec) Clone() AgentPythonExec {
	e.CodeSummary = e.CodeSummary.Clone()
	e.ResourceSummary = e.ResourceSummary.Clone()
	e.StdoutSummary = e.StdoutSummary.Clone()
	e.StderrSummary = e.StderrSummary.Clone()
	e.ResultSummary = e.ResultSummary.Clone()
	return e
}

type CreateRunInput struct {
	RunID            string
	AgentID          string
	ConversationID   string
	TriggerMessageID string
	RequestingUserID string
	Status           Status
	InputSummary     Summary
	OutputSummary    Summary
	OutputMessageID  string
	ErrorCode        string
	ErrorMessage     string
	TraceID          string
	RequestID        string
	StartedAt        time.Time
	FinishedAt       time.Time
}

type CreateToolCallInput struct {
	ToolCallID    string
	RunID         string
	AgentID       string
	ToolID        string
	ToolName      string
	Status        Status
	InputSummary  Summary
	OutputSummary Summary
	DurationMs    int64
	ErrorCode     string
	ErrorMessage  string
	TraceID       string
	RequestID     string
	StartedAt     time.Time
	FinishedAt    time.Time
}

type CreateFileReadInput struct {
	FileReadID     string
	RunID          string
	AgentID        string
	SkillID        string
	FileID         string
	ObjectKey      string
	SHA256         string
	Status         Status
	ByteCount      int64
	ContentSummary Summary
	ErrorCode      string
	ErrorMessage   string
	TraceID        string
	RequestID      string
	StartedAt      time.Time
	FinishedAt     time.Time
}

type CreatePythonExecInput struct {
	PythonExecID     string
	RunID            string
	AgentID          string
	SandboxRequestID string
	Status           Status
	Code             string
	CodeSummary      Summary
	ResourceSummary  Summary
	StdoutSummary    Summary
	StderrSummary    Summary
	ResultSummary    Summary
	ErrorCode        string
	ErrorMessage     string
	TraceID          string
	RequestID        string
	StartedAt        time.Time
	FinishedAt       time.Time
}

func NormalizeCreateRunInput(input CreateRunInput) (CreateRunInput, error) {
	input.RunID = strings.TrimSpace(input.RunID)
	input.AgentID = strings.TrimSpace(input.AgentID)
	input.ConversationID = strings.TrimSpace(input.ConversationID)
	input.TriggerMessageID = strings.TrimSpace(input.TriggerMessageID)
	input.RequestingUserID = strings.TrimSpace(input.RequestingUserID)
	input.OutputMessageID = strings.TrimSpace(input.OutputMessageID)
	input.TraceID = strings.TrimSpace(input.TraceID)
	input.RequestID = strings.TrimSpace(input.RequestID)
	if input.AgentID == "" {
		return CreateRunInput{}, apperror.InvalidArgument("agent_id is required")
	}
	status, err := NormalizeStatus(input.Status)
	if err != nil {
		return CreateRunInput{}, err
	}
	input.Status = status
	input.ErrorCode, input.ErrorMessage, err = normalizeError(input.Status, input.ErrorCode, input.ErrorMessage)
	if err != nil {
		return CreateRunInput{}, err
	}
	input.InputSummary, err = RedactSummary(input.InputSummary)
	if err != nil {
		return CreateRunInput{}, err
	}
	input.OutputSummary, err = RedactSummary(input.OutputSummary)
	if err != nil {
		return CreateRunInput{}, err
	}
	input.StartedAt = utcOrZero(input.StartedAt)
	input.FinishedAt = utcOrZero(input.FinishedAt)
	return input, nil
}

func NormalizeCreateToolCallInput(input CreateToolCallInput) (CreateToolCallInput, error) {
	input.ToolCallID = strings.TrimSpace(input.ToolCallID)
	input.RunID = strings.TrimSpace(input.RunID)
	input.AgentID = strings.TrimSpace(input.AgentID)
	input.ToolID = strings.TrimSpace(input.ToolID)
	input.ToolName = strings.TrimSpace(input.ToolName)
	input.TraceID = strings.TrimSpace(input.TraceID)
	input.RequestID = strings.TrimSpace(input.RequestID)
	if input.RunID == "" {
		return CreateToolCallInput{}, apperror.InvalidArgument("run_id is required")
	}
	if input.AgentID == "" {
		return CreateToolCallInput{}, apperror.InvalidArgument("agent_id is required")
	}
	if input.ToolName == "" {
		return CreateToolCallInput{}, apperror.InvalidArgument("tool_name is required")
	}
	status, err := NormalizeStatus(input.Status)
	if err != nil {
		return CreateToolCallInput{}, err
	}
	input.Status = status
	input.ErrorCode, input.ErrorMessage, err = normalizeError(input.Status, input.ErrorCode, input.ErrorMessage)
	if err != nil {
		return CreateToolCallInput{}, err
	}
	input.InputSummary, err = RedactSummary(input.InputSummary)
	if err != nil {
		return CreateToolCallInput{}, err
	}
	input.OutputSummary, err = RedactSummary(input.OutputSummary)
	if err != nil {
		return CreateToolCallInput{}, err
	}
	if input.DurationMs < 0 {
		return CreateToolCallInput{}, apperror.InvalidArgument("duration_ms cannot be negative")
	}
	input.StartedAt = utcOrZero(input.StartedAt)
	input.FinishedAt = utcOrZero(input.FinishedAt)
	return input, nil
}

func NormalizeCreateFileReadInput(input CreateFileReadInput) (CreateFileReadInput, error) {
	input.FileReadID = strings.TrimSpace(input.FileReadID)
	input.RunID = strings.TrimSpace(input.RunID)
	input.AgentID = strings.TrimSpace(input.AgentID)
	input.SkillID = strings.TrimSpace(input.SkillID)
	input.FileID = strings.TrimSpace(input.FileID)
	input.ObjectKey = strings.TrimSpace(input.ObjectKey)
	input.SHA256 = strings.TrimSpace(input.SHA256)
	input.TraceID = strings.TrimSpace(input.TraceID)
	input.RequestID = strings.TrimSpace(input.RequestID)
	if input.RunID == "" {
		return CreateFileReadInput{}, apperror.InvalidArgument("run_id is required")
	}
	if input.AgentID == "" {
		return CreateFileReadInput{}, apperror.InvalidArgument("agent_id is required")
	}
	if input.SkillID == "" {
		return CreateFileReadInput{}, apperror.InvalidArgument("skill_id is required")
	}
	if input.ObjectKey == "" {
		return CreateFileReadInput{}, apperror.InvalidArgument("object_key is required")
	}
	status, err := NormalizeStatus(input.Status)
	if err != nil {
		return CreateFileReadInput{}, err
	}
	input.Status = status
	input.ErrorCode, input.ErrorMessage, err = normalizeError(input.Status, input.ErrorCode, input.ErrorMessage)
	if err != nil {
		return CreateFileReadInput{}, err
	}
	input.ContentSummary, err = RedactSummary(input.ContentSummary)
	if err != nil {
		return CreateFileReadInput{}, err
	}
	if input.ByteCount < 0 {
		return CreateFileReadInput{}, apperror.InvalidArgument("byte_count cannot be negative")
	}
	input.StartedAt = utcOrZero(input.StartedAt)
	input.FinishedAt = utcOrZero(input.FinishedAt)
	return input, nil
}

func NormalizeCreatePythonExecInput(input CreatePythonExecInput) (CreatePythonExecInput, error) {
	input.PythonExecID = strings.TrimSpace(input.PythonExecID)
	input.RunID = strings.TrimSpace(input.RunID)
	input.AgentID = strings.TrimSpace(input.AgentID)
	input.SandboxRequestID = strings.TrimSpace(input.SandboxRequestID)
	input.TraceID = strings.TrimSpace(input.TraceID)
	input.RequestID = strings.TrimSpace(input.RequestID)
	if input.RunID == "" {
		return CreatePythonExecInput{}, apperror.InvalidArgument("run_id is required")
	}
	if input.AgentID == "" {
		return CreatePythonExecInput{}, apperror.InvalidArgument("agent_id is required")
	}
	status, err := NormalizeStatus(input.Status)
	if err != nil {
		return CreatePythonExecInput{}, err
	}
	input.Status = status
	input.ErrorCode, input.ErrorMessage, err = normalizeError(input.Status, input.ErrorCode, input.ErrorMessage)
	if err != nil {
		return CreatePythonExecInput{}, err
	}
	if input.Code != "" {
		input.CodeSummary = SummarizePythonCode(input.Code)
		input.Code = ""
	} else {
		input.CodeSummary, err = RedactSummary(input.CodeSummary)
		if err != nil {
			return CreatePythonExecInput{}, err
		}
	}
	input.ResourceSummary, err = RedactSummary(input.ResourceSummary)
	if err != nil {
		return CreatePythonExecInput{}, err
	}
	input.StdoutSummary, err = RedactSummary(input.StdoutSummary)
	if err != nil {
		return CreatePythonExecInput{}, err
	}
	input.StderrSummary, err = RedactSummary(input.StderrSummary)
	if err != nil {
		return CreatePythonExecInput{}, err
	}
	input.ResultSummary, err = RedactSummary(input.ResultSummary)
	if err != nil {
		return CreatePythonExecInput{}, err
	}
	input.StartedAt = utcOrZero(input.StartedAt)
	input.FinishedAt = utcOrZero(input.FinishedAt)
	return input, nil
}

func NormalizeStatus(status Status) (Status, error) {
	switch status {
	case StatusStarted, StatusSucceeded, StatusFailed, StatusCancelled:
		return status, nil
	default:
		return "", apperror.InvalidArgument("agent audit status is invalid")
	}
}

func RedactSummary(summary Summary) (Summary, error) {
	if len(summary) == 0 {
		return Summary{}, nil
	}
	redacted, ok, err := redactAny("", map[string]any(summary))
	if err != nil {
		return Summary{}, err
	}
	if !ok {
		return Summary{}, apperror.InvalidArgument("summary must be a JSON object")
	}
	result, ok := redacted.(map[string]any)
	if !ok {
		return Summary{}, apperror.InvalidArgument("summary must be a JSON object")
	}
	return Summary(result), nil
}

func SummarizePythonCode(code string) Summary {
	sum := sha256.Sum256([]byte(code))
	return Summary{
		"sha256":     hex.EncodeToString(sum[:]),
		"size_bytes": len([]byte(code)),
	}
}

func RedactPlainText(value string) string {
	value = strings.TrimSpace(value)
	for _, pattern := range inlineSecretPatterns {
		value = pattern.ReplaceAllString(value, "${1}"+RedactedValue)
	}
	if len(value) <= maxAuditTextLength {
		return value
	}
	sum := sha256.Sum256([]byte(value))
	return fmt.Sprintf("sha256:%s size_bytes:%d", hex.EncodeToString(sum[:]), len([]byte(value)))
}

func normalizeError(status Status, code string, message string) (string, string, error) {
	code = RedactPlainText(code)
	message = RedactPlainText(message)
	if status == StatusFailed && code == "" && message == "" {
		return "", "", apperror.InvalidArgument("failed audit records require error_code or error_message")
	}
	return code, message, nil
}

func utcOrZero(value time.Time) time.Time {
	if value.IsZero() {
		return time.Time{}
	}
	return value.UTC()
}

func redactAny(key string, value any) (any, bool, error) {
	if isSensitiveKey(key) {
		return RedactedValue, true, nil
	}
	switch typed := value.(type) {
	case nil:
		return nil, true, nil
	case string:
		return RedactPlainText(typed), true, nil
	case bool:
		return typed, true, nil
	case int:
		return typed, true, nil
	case int64:
		return typed, true, nil
	case int32:
		return typed, true, nil
	case float64:
		return typed, true, nil
	case float32:
		return typed, true, nil
	case json.Number:
		return typed, true, nil
	case Summary:
		return redactMap(map[string]any(typed))
	case map[string]any:
		return redactMap(typed)
	case []any:
		items := make([]any, 0, len(typed))
		for _, item := range typed {
			redacted, _, err := redactAny("", item)
			if err != nil {
				return nil, false, err
			}
			items = append(items, redacted)
		}
		return items, true, nil
	case []string:
		items := make([]any, 0, len(typed))
		for _, item := range typed {
			items = append(items, RedactPlainText(item))
		}
		return items, true, nil
	default:
		encoded, err := json.Marshal(typed)
		if err != nil {
			return nil, false, apperror.InvalidArgument("summary contains non-json value")
		}
		var decoded any
		if err := json.Unmarshal(encoded, &decoded); err != nil {
			return nil, false, apperror.InvalidArgument("summary contains invalid json value")
		}
		return redactAny(key, decoded)
	}
}

func redactMap(values map[string]any) (any, bool, error) {
	result := make(map[string]any, len(values))
	for key, value := range values {
		redacted, _, err := redactAny(key, value)
		if err != nil {
			return nil, false, err
		}
		result[key] = redacted
	}
	return result, true, nil
}

func isSensitiveKey(key string) bool {
	normalized := strings.ToLower(strings.TrimSpace(key))
	normalized = strings.ReplaceAll(normalized, "-", "_")
	normalized = strings.ReplaceAll(normalized, ".", "_")
	switch {
	case strings.Contains(normalized, "password"):
		return true
	case strings.Contains(normalized, "token"):
		return true
	case strings.Contains(normalized, "secret"):
		return true
	case strings.Contains(normalized, "credential"):
		return true
	case strings.Contains(normalized, "authorization"):
		return true
	case strings.Contains(normalized, "api_key"):
		return true
	case strings.Contains(normalized, "apikey"):
		return true
	case strings.Contains(normalized, "access_key"):
		return true
	case strings.Contains(normalized, "private_key"):
		return true
	case strings.Contains(normalized, "cookie"):
		return true
	case strings.Contains(normalized, "session"):
		return true
	default:
		return false
	}
}
