package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/wujunhui99/agents_im/pkg/model"
	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/pkg/pythonexec"
)

const (
	defaultPythonExecuteTimeout        = 10 * time.Second
	maxPythonExecuteTimeout            = 30 * time.Second
	defaultPythonExecuteMemoryBytes    = 256 * 1024 * 1024
	defaultPythonExecuteMaxOutputBytes = 64 * 1024
)

type PythonExecuteAdapterOption func(*pythonExecuteAdapterConfig)

type pythonExecuteAdapterConfig struct {
	defaultTimeout time.Duration
	maxTimeout     time.Duration
	memoryLimit    int64
	maxOutputBytes int64
	fileAllowlist  map[string]pythonexec.FileAllowlistEntry
}

type PythonExecuteAdapter struct {
	spec     ToolSpec
	executor pythonexec.Executor
	config   pythonExecuteAdapterConfig
}

func NewPythonExecuteAdapter(spec ToolSpec, executor pythonexec.Executor, opts ...PythonExecuteAdapterOption) (*PythonExecuteAdapter, error) {
	if !isPythonExecuteToolSpec(spec) {
		return nil, apperror.InvalidArgument("python execute adapter requires a local python.execute tool spec")
	}
	if executor == nil {
		executor = pythonexec.NewDefaultExecutor()
	}
	config := defaultPythonExecuteAdapterConfig()
	for _, opt := range opts {
		if opt != nil {
			opt(&config)
		}
	}
	if config.defaultTimeout <= 0 {
		return nil, apperror.InvalidArgument("python execute default timeout must be greater than zero")
	}
	if config.maxTimeout <= 0 || config.defaultTimeout > config.maxTimeout {
		return nil, apperror.InvalidArgument("python execute max timeout must be greater than or equal to default timeout")
	}
	if config.memoryLimit <= 0 {
		return nil, apperror.InvalidArgument("python execute memory limit must be greater than zero")
	}
	if config.maxOutputBytes <= 0 {
		return nil, apperror.InvalidArgument("python execute max output bytes must be greater than zero")
	}
	return &PythonExecuteAdapter{spec: spec, executor: executor, config: config}, nil
}

func WithPythonExecuteFileAllowlist(entries map[string]pythonexec.FileAllowlistEntry) PythonExecuteAdapterOption {
	return func(config *pythonExecuteAdapterConfig) {
		config.fileAllowlist = make(map[string]pythonexec.FileAllowlistEntry, len(entries))
		for key, entry := range entries {
			normalized := strings.TrimSpace(key)
			if normalized != "" {
				config.fileAllowlist[normalized] = entry
			}
		}
	}
}

func (a *PythonExecuteAdapter) Spec() ToolSpec {
	if a == nil {
		return ToolSpec{}
	}
	return a.spec
}

func (a *PythonExecuteAdapter) Invoke(ctx context.Context, call ToolCall) (ToolResult, error) {
	if a == nil {
		return ToolResult{}, apperror.Internal("python execute adapter is nil")
	}
	if ctx == nil {
		return ToolResult{}, apperror.InvalidArgument("context is required")
	}
	if strings.TrimSpace(call.ToolID) != a.spec.ToolID {
		return ToolResult{}, apperror.InvalidArgument("tool_id does not match python execute adapter")
	}

	input, err := decodePythonExecuteInput(call.InputJSON)
	if err != nil {
		return ToolResult{}, err
	}
	if strings.TrimSpace(input.Code) == "" {
		return ToolResult{}, apperror.InvalidArgument("code is required")
	}
	timeout, err := a.timeout(input.TimeoutSeconds)
	if err != nil {
		return ToolResult{}, err
	}
	fileAllowlist, err := a.fileAllowlist(input.Files)
	if err != nil {
		return ToolResult{}, err
	}

	req := pythonexec.Request{
		Code: input.Code,
		Policy: pythonexec.Policy{
			RunID:            strings.TrimSpace(call.RunID),
			AuditID:          pythonExecuteAuditID(call),
			Timeout:          timeout,
			CPUTimeLimit:     timeout,
			MemoryLimitBytes: a.config.memoryLimit,
			Network:          pythonexec.NetworkPolicyDisabled,
			FileAllowlist:    fileAllowlist,
			MaxOutputBytes:   a.config.maxOutputBytes,
		},
	}
	if err := req.Validate(); err != nil {
		return ToolResult{}, err
	}

	resp, err := a.executor.Execute(ctx, req)
	if err != nil {
		return ToolResult{}, err
	}
	if resp == nil {
		return ToolResult{}, apperror.Internal("python executor returned nil response")
	}
	output, err := marshalPythonExecuteOutput(resp)
	if err != nil {
		return ToolResult{}, err
	}
	return ToolResult{OutputJSON: output}, nil
}

type pythonExecuteInput struct {
	Code           string   `json:"code"`
	TimeoutSeconds int      `json:"timeout_seconds"`
	Files          []string `json:"files"`
}

type pythonExecuteOutput struct {
	Stdout          string                    `json:"stdout"`
	Stderr          string                    `json:"stderr"`
	ResultJSON      json.RawMessage           `json:"result_json"`
	ExitCode        int                       `json:"exit_code"`
	TimedOut        bool                      `json:"timed_out"`
	OutputTruncated bool                      `json:"output_truncated"`
	Error           *pythonExecuteOutputError `json:"error"`
}

type pythonExecuteOutputError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func defaultPythonExecuteAdapterConfig() pythonExecuteAdapterConfig {
	return pythonExecuteAdapterConfig{
		defaultTimeout: defaultPythonExecuteTimeout,
		maxTimeout:     maxPythonExecuteTimeout,
		memoryLimit:    defaultPythonExecuteMemoryBytes,
		maxOutputBytes: defaultPythonExecuteMaxOutputBytes,
		fileAllowlist:  map[string]pythonexec.FileAllowlistEntry{},
	}
}

func isPythonExecuteToolSpec(spec ToolSpec) bool {
	return spec.ToolType == model.AgentToolTypeLocal &&
		spec.Local != nil &&
		strings.TrimSpace(spec.Local.HandlerKey) == model.LocalToolHandlerPythonExecute
}

func decodePythonExecuteInput(raw json.RawMessage) (pythonExecuteInput, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return pythonExecuteInput{}, apperror.InvalidArgument("input_json is required")
	}
	var input pythonExecuteInput
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		return pythonExecuteInput{}, apperror.InvalidArgument("python execute input_json is invalid: " + err.Error())
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return pythonExecuteInput{}, apperror.InvalidArgument("python execute input_json must contain a single JSON object")
	}
	return input, nil
}

func (a *PythonExecuteAdapter) timeout(timeoutSeconds int) (time.Duration, error) {
	if timeoutSeconds < 0 {
		return 0, apperror.InvalidArgument("timeout_seconds must be non-negative")
	}
	if timeoutSeconds == 0 {
		return a.config.defaultTimeout, nil
	}
	timeout := time.Duration(timeoutSeconds) * time.Second
	if timeout > a.config.maxTimeout {
		return 0, apperror.InvalidArgument(fmt.Sprintf("timeout_seconds must be less than or equal to %.0f", a.config.maxTimeout.Seconds()))
	}
	return timeout, nil
}

func (a *PythonExecuteAdapter) fileAllowlist(files []string) ([]pythonexec.FileAllowlistEntry, error) {
	if len(files) == 0 {
		return []pythonexec.FileAllowlistEntry{}, nil
	}
	allowlist := make([]pythonexec.FileAllowlistEntry, 0, len(files))
	for _, raw := range files {
		file := strings.TrimSpace(raw)
		if file == "" {
			return nil, apperror.InvalidArgument("files cannot contain empty paths")
		}
		entry, ok := a.config.fileAllowlist[file]
		if !ok {
			return nil, apperror.Forbidden("python execute file is not allowlisted")
		}
		if err := entry.Validate(); err != nil {
			return nil, apperror.InvalidArgument("python execute file allowlist is invalid: " + err.Error())
		}
		allowlist = append(allowlist, entry)
	}
	return allowlist, nil
}

func pythonExecuteAuditID(call ToolCall) string {
	if requestID := strings.TrimSpace(call.RequestID); requestID != "" {
		return requestID
	}
	if traceID := strings.TrimSpace(call.TraceID); traceID != "" {
		return traceID
	}
	runID := strings.TrimSpace(call.RunID)
	toolID := strings.TrimSpace(call.ToolID)
	if runID != "" && toolID != "" {
		return runID + ":" + toolID
	}
	return runID
}

func marshalPythonExecuteOutput(resp *pythonexec.Response) (json.RawMessage, error) {
	resultJSON := json.RawMessage(resp.ResultJSON)
	if len(resultJSON) == 0 {
		resultJSON = json.RawMessage("null")
	}
	output := pythonExecuteOutput{
		Stdout:          resp.Stdout,
		Stderr:          resp.Stderr,
		ResultJSON:      resultJSON,
		ExitCode:        resp.ExitCode,
		TimedOut:        resp.TimedOut,
		OutputTruncated: resp.OutputTruncated,
	}
	if resp.Error != nil {
		output.Error = &pythonExecuteOutputError{
			Code:    resp.Error.Code,
			Message: resp.Error.Message,
		}
	}
	encoded, err := json.Marshal(output)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(encoded), nil
}
