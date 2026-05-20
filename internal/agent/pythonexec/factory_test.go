package pythonexec

import (
	"testing"

	"github.com/wujunhui99/agents_im/internal/config"
)

func TestNewExecutorFromConfigDefaultsDisabled(t *testing.T) {
	executor, err := NewExecutorFromConfig(config.PythonExecutorConfig{}, nil)
	if err != nil {
		t.Fatalf("new executor from default config: %v", err)
	}
	if _, ok := executor.(*DisabledExecutor); !ok {
		t.Fatalf("default python executor should be disabled, got %T", executor)
	}
}

func TestNewExecutorFromConfigBuildsKubernetesWithInjectedClient(t *testing.T) {
	executor, err := NewExecutorFromConfig(config.PythonExecutorConfig{
		Backend: config.PythonExecutorBackendK8S,
		K8S: config.PythonExecutorK8SConfig{
			Namespace: "agent-python-sandbox",
			Image:     "ghcr.io/wujunhui99/agents_im/python-sandbox:test",
		},
		DefaultTimeoutSeconds: 10,
		MaxTimeoutSeconds:     30,
		DefaultMemoryMiB:      128,
		MaxMemoryMiB:          256,
		MaxOutputBytes:        64 * 1024,
	}, &fakeKubernetesSandboxClient{})
	if err != nil {
		t.Fatalf("new k8s executor from config: %v", err)
	}
	k8sExecutor, ok := executor.(*KubernetesExecutor)
	if !ok {
		t.Fatalf("expected kubernetes executor, got %T", executor)
	}
	if k8sExecutor.config.ImagePullSecret != "ghcr-pull-secret" {
		t.Fatalf("kubernetes executor image pull secret = %q, want ghcr-pull-secret", k8sExecutor.config.ImagePullSecret)
	}
}
