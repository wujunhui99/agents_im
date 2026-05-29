package pythonexec

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestDisabledExecutorFailsExplicitly(t *testing.T) {
	executor := NewDisabledExecutor()

	resp, err := executor.Execute(context.Background(), Request{
		Code:   "print(1 + 1)",
		Policy: validPolicy(),
	})

	if resp != nil {
		t.Fatalf("disabled executor returned response: %#v", resp)
	}
	if !errors.Is(err, ErrPythonExecutorDisabled) {
		t.Fatalf("expected ErrPythonExecutorDisabled, got %v", err)
	}
}

func TestPolicyValidationRejectsMissingOrUnsafeValues(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Policy)
		want   string
	}{
		{
			name: "missing run id",
			mutate: func(policy *Policy) {
				policy.RunID = ""
			},
			want: "run_id",
		},
		{
			name: "missing audit id",
			mutate: func(policy *Policy) {
				policy.AuditID = ""
			},
			want: "audit_id",
		},
		{
			name: "missing timeout",
			mutate: func(policy *Policy) {
				policy.Timeout = 0
			},
			want: "timeout",
		},
		{
			name: "missing cpu limit",
			mutate: func(policy *Policy) {
				policy.CPUTimeLimit = 0
			},
			want: "cpu",
		},
		{
			name: "missing memory limit",
			mutate: func(policy *Policy) {
				policy.MemoryLimitBytes = 0
			},
			want: "memory",
		},
		{
			name: "network enabled",
			mutate: func(policy *Policy) {
				policy.Network = NetworkPolicyOutboundAllowed
			},
			want: "network",
		},
		{
			name: "missing max output bytes",
			mutate: func(policy *Policy) {
				policy.MaxOutputBytes = 0
			},
			want: "max_output_bytes",
		},
		{
			name: "file allowlist must be explicit",
			mutate: func(policy *Policy) {
				policy.FileAllowlist = nil
			},
			want: "file_allowlist",
		},
		{
			name: "absolute allowlist path",
			mutate: func(policy *Policy) {
				policy.FileAllowlist = []FileAllowlistEntry{{Path: "/etc/passwd", ReadOnly: true}}
			},
			want: "file_allowlist",
		},
		{
			name: "escaping allowlist path",
			mutate: func(policy *Policy) {
				policy.FileAllowlist = []FileAllowlistEntry{{Path: "../secret.txt", ReadOnly: true}}
			},
			want: "file_allowlist",
		},
		{
			name: "writable allowlist path",
			mutate: func(policy *Policy) {
				policy.FileAllowlist = []FileAllowlistEntry{{Path: "input.txt", ReadOnly: false}}
			},
			want: "read_only",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := validPolicy()
			tt.mutate(&policy)

			err := policy.Validate()
			if err == nil {
				t.Fatalf("expected validation error containing %q", tt.want)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected validation error containing %q, got %v", tt.want, err)
			}
		})
	}
}

func TestRequestValidationRequiresCodeAndPolicy(t *testing.T) {
	req := Request{
		Code:   "print(1 + 1)",
		Policy: validPolicy(),
	}
	if err := req.Validate(); err != nil {
		t.Fatalf("valid request rejected: %v", err)
	}

	req.Code = ""
	if err := req.Validate(); err == nil || !strings.Contains(err.Error(), "code") {
		t.Fatalf("expected code validation error, got %v", err)
	}
}

func TestSandboxContractHasNoShellCommandSurface(t *testing.T) {
	assertTypeHasNoForbiddenFields(t, reflect.TypeOf(Request{}))
	assertTypeHasNoForbiddenFields(t, reflect.TypeOf(Policy{}))
	assertTypeHasNoForbiddenFields(t, reflect.TypeOf(Response{}))

	executorType := reflect.TypeOf((*Executor)(nil)).Elem()
	if executorType.NumMethod() != 1 {
		t.Fatalf("executor interface should expose only Execute, got %d methods", executorType.NumMethod())
	}
	if method := executorType.Method(0); method.Name != "Execute" {
		t.Fatalf("executor method should be Execute, got %s", method.Name)
	}
}

func assertTypeHasNoForbiddenFields(t *testing.T, typ reflect.Type) {
	t.Helper()

	forbidden := map[string]struct{}{
		"args":       {},
		"binary":     {},
		"command":    {},
		"entrypoint": {},
		"shell":      {},
	}

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		name := strings.ToLower(field.Name)
		if _, ok := forbidden[name]; ok {
			t.Fatalf("%s exposes forbidden command/shell field %q", typ.Name(), field.Name)
		}
	}
}

func validPolicy() Policy {
	return Policy{
		RunID:            "run-123",
		AuditID:          "audit-123",
		Timeout:          10 * time.Second,
		CPUTimeLimit:     2 * time.Second,
		MemoryLimitBytes: 128 * 1024 * 1024,
		Network:          NetworkPolicyDisabled,
		FileAllowlist:    []FileAllowlistEntry{},
		MaxOutputBytes:   64 * 1024,
	}
}
