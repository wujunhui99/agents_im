package pythonexec

import (
	"fmt"
	"time"

	"github.com/wujunhui99/agents_im/internal/config"
)

func NewExecutorFromConfig(executorConfig config.PythonExecutorConfig, client KubernetesSandboxClient) (Executor, error) {
	resolved, err := config.ResolvePythonExecutorConfig(executorConfig)
	if err != nil {
		return nil, err
	}
	switch resolved.Backend {
	case config.PythonExecutorBackendDisabled:
		return NewDisabledExecutor(), nil
	case config.PythonExecutorBackendK8S:
		if client == nil {
			return nil, fmt.Errorf("%w: kubernetes sandbox client is required for k8s backend", ErrInvalidPolicy)
		}
		return NewKubernetesExecutor(KubernetesExecutorConfig{
			Namespace:          resolved.K8S.Namespace,
			Image:              resolved.K8S.Image,
			ServiceAccountName: resolved.K8S.ServiceAccountName,
			RuntimeClassName:   resolved.K8S.RuntimeClassName,
			ImagePullSecret:    "ghcr-pull-secret",
			MaxTimeout:         time.Duration(resolved.MaxTimeoutSeconds) * time.Second,
			MaxMemoryBytes:     int64(resolved.MaxMemoryMiB) * 1024 * 1024,
			MaxOutputBytes:     resolved.MaxOutputBytes,
		}, client)
	default:
		return nil, fmt.Errorf("%w: unsupported python executor backend %q", ErrInvalidPolicy, resolved.Backend)
	}
}
