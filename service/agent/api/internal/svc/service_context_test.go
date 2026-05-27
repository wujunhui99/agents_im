package svc

import (
	"testing"

	"github.com/wujunhui99/agents_im/internal/agent/pythonexec"
	"github.com/wujunhui99/agents_im/internal/config"
	"github.com/wujunhui99/agents_im/internal/repository"
)

func TestServiceContextDefaultsPythonExecutorDisabled(t *testing.T) {
	ctx := NewServiceContextWithAuth(repository.NewMemoryAgentRepository(), nil, config.DefaultJWTAuthConfig())
	if _, ok := ctx.PythonExecutor.(*pythonexec.DisabledExecutor); !ok {
		t.Fatalf("default python executor should be disabled, got %T", ctx.PythonExecutor)
	}
}

func TestServiceContextAcceptsConfiguredPythonExecutor(t *testing.T) {
	executor := pythonexec.NewDisabledExecutor()
	ctx := NewServiceContextWithAuthAndPythonExecutor(repository.NewMemoryAgentRepository(), nil, config.DefaultJWTAuthConfig(), executor)
	if ctx.PythonExecutor != executor {
		t.Fatalf("python executor injection mismatch: got %T", ctx.PythonExecutor)
	}
}
