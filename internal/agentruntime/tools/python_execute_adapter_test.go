package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/wujunhui99/agents_im/pkg/model"
	"github.com/wujunhui99/agents_im/pkg/pythonexec"
)

func TestPythonExecuteAdapterInvokesExecutorWithSafePolicyAndReturnsJSON(t *testing.T) {
	spec := validPythonExecuteToolSpec()
	executor := &fakePythonExecutor{
		resp: &pythonexec.Response{
			RunID:           "run-123",
			AuditID:         "req-123",
			Stdout:          "2\n",
			Stderr:          "",
			ResultJSON:      []byte(`{"value":2}`),
			ExitCode:        0,
			TimedOut:        false,
			OutputTruncated: false,
		},
	}
	adapter, err := NewPythonExecuteAdapter(spec, executor)
	if err != nil {
		t.Fatal(err)
	}

	result, err := adapter.Invoke(context.Background(), ToolCall{
		RunID:     "run-123",
		AgentID:   "agent_support",
		ToolID:    spec.ToolID,
		ToolName:  spec.Name,
		InputJSON: json.RawMessage(`{"code":"print(1 + 1)","timeout_seconds":5,"files":[]}`),
		TraceID:   "trace-123",
		RequestID: "req-123",
	})
	if err != nil {
		t.Fatal(err)
	}
	if executor.calls != 1 {
		t.Fatalf("executor calls = %d, want 1", executor.calls)
	}
	if executor.lastReq.Code != "print(1 + 1)" {
		t.Fatalf("code mismatch: %q", executor.lastReq.Code)
	}
	policy := executor.lastReq.Policy
	if policy.RunID != "run-123" || policy.AuditID != "req-123" {
		t.Fatalf("policy ids mismatch: %+v", policy)
	}
	if policy.Timeout != 5*time.Second || policy.CPUTimeLimit != policy.Timeout {
		t.Fatalf("policy timeout mismatch: %+v", policy)
	}
	if policy.MemoryLimitBytes != 256*1024*1024 || policy.MaxOutputBytes != 64*1024 {
		t.Fatalf("policy resource limits mismatch: %+v", policy)
	}
	if policy.EffectiveNetworkPolicy() != pythonexec.NetworkPolicyDisabled {
		t.Fatalf("network policy mismatch: %+v", policy.Network)
	}
	if len(policy.FileAllowlist) != 0 {
		t.Fatalf("file allowlist should be empty by default: %+v", policy.FileAllowlist)
	}

	var output struct {
		Stdout          string          `json:"stdout"`
		Stderr          string          `json:"stderr"`
		ResultJSON      json.RawMessage `json:"result_json"`
		ExitCode        int             `json:"exit_code"`
		TimedOut        bool            `json:"timed_out"`
		OutputTruncated bool            `json:"output_truncated"`
		Error           any             `json:"error"`
	}
	if err := json.Unmarshal(result.OutputJSON, &output); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if output.Stdout != "2\n" || output.Stderr != "" || string(output.ResultJSON) != `{"value":2}` {
		t.Fatalf("output mismatch: %+v raw=%s", output, string(result.OutputJSON))
	}
	if output.ExitCode != 0 || output.TimedOut || output.OutputTruncated || output.Error != nil {
		t.Fatalf("execution metadata mismatch: %+v raw=%s", output, string(result.OutputJSON))
	}
}

func TestPythonExecuteAdapterRejectsMissingCodeBeforeExecutorCall(t *testing.T) {
	executor := &fakePythonExecutor{}
	adapter, err := NewPythonExecuteAdapter(validPythonExecuteToolSpec(), executor)
	if err != nil {
		t.Fatal(err)
	}

	_, err = adapter.Invoke(context.Background(), ToolCall{
		RunID:     "run-123",
		AgentID:   "agent_support",
		ToolID:    "tool_python",
		ToolName:  model.LocalToolHandlerPythonExecute,
		InputJSON: json.RawMessage(`{"code":"   "}`),
		RequestID: "req-123",
	})
	if err == nil || !strings.Contains(err.Error(), "code is required") {
		t.Fatalf("expected missing code error, got %v", err)
	}
	if executor.calls != 0 {
		t.Fatalf("executor should not be called, got %d calls", executor.calls)
	}
}

func TestPythonExecuteAdapterDisabledExecutorReturnsVisibleError(t *testing.T) {
	adapter, err := NewPythonExecuteAdapter(validPythonExecuteToolSpec(), pythonexec.NewDisabledExecutor())
	if err != nil {
		t.Fatal(err)
	}

	_, err = adapter.Invoke(context.Background(), ToolCall{
		RunID:     "run-123",
		AgentID:   "agent_support",
		ToolID:    "tool_python",
		ToolName:  model.LocalToolHandlerPythonExecute,
		InputJSON: json.RawMessage(`{"code":"print(1 + 1)"}`),
		RequestID: "req-123",
	})
	if err == nil || !strings.Contains(err.Error(), "python executor is disabled") {
		t.Fatalf("expected disabled executor error, got %v", err)
	}
}

func validPythonExecuteToolSpec() ToolSpec {
	return ToolSpec{
		ToolID:           "tool_python",
		Name:             model.LocalToolHandlerPythonExecute,
		ToolType:         model.AgentToolTypeLocal,
		InputSchemaJSON:  `{"type":"object"}`,
		OutputSchemaJSON: `{"type":"object"}`,
		PermissionLevel:  "agent_bound",
		Local:            &LocalToolSpec{HandlerKey: model.LocalToolHandlerPythonExecute},
	}
}

type fakePythonExecutor struct {
	calls   int
	lastReq pythonexec.Request
	resp    *pythonexec.Response
	err     error
}

func (e *fakePythonExecutor) Execute(ctx context.Context, req pythonexec.Request) (*pythonexec.Response, error) {
	if ctx == nil {
		panic("nil context")
	}
	e.calls++
	e.lastReq = req
	return e.resp, e.err
}
