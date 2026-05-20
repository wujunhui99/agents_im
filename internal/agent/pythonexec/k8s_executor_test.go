package pythonexec

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
)

func TestKubernetesManifestAppliesSandboxSecurityControls(t *testing.T) {
	req := validKubernetesRequest()
	req.Code = "print('do not leak code into metadata')"

	workload, err := buildKubernetesSandboxWorkload(validKubernetesExecutorConfig(), req)
	if err != nil {
		t.Fatalf("build workload: %v", err)
	}

	if workload.CodeConfigMap == nil || workload.Job == nil {
		t.Fatalf("expected job and configmap workload, got %+v", workload)
	}
	if strings.Contains(workload.Job.Name, "print") || strings.Contains(workload.CodeConfigMap.Name, "print") {
		t.Fatalf("workload names must not contain raw code: job=%q configmap=%q", workload.Job.Name, workload.CodeConfigMap.Name)
	}

	podSpec := workload.Job.Spec.Template.Spec
	if podSpec.AutomountServiceAccountToken == nil || *podSpec.AutomountServiceAccountToken {
		t.Fatalf("sandbox pod must disable service account token automount: %+v", podSpec.AutomountServiceAccountToken)
	}
	if podSpec.HostNetwork || podSpec.HostPID || podSpec.HostIPC {
		t.Fatalf("sandbox pod must not request host namespaces: %+v", podSpec)
	}
	if podSpec.RestartPolicy != corev1.RestartPolicyNever {
		t.Fatalf("restart policy = %q, want Never", podSpec.RestartPolicy)
	}
	if podSpec.SecurityContext == nil || podSpec.SecurityContext.RunAsNonRoot == nil || !*podSpec.SecurityContext.RunAsNonRoot {
		t.Fatalf("pod security context must run as non-root: %+v", podSpec.SecurityContext)
	}
	for _, volume := range podSpec.Volumes {
		if volume.HostPath != nil {
			t.Fatalf("sandbox workload must not include hostPath volume: %+v", volume)
		}
	}

	if len(podSpec.Containers) != 1 {
		t.Fatalf("container count = %d, want 1", len(podSpec.Containers))
	}
	container := podSpec.Containers[0]
	if len(container.Command) != 0 || len(container.Args) != 0 {
		t.Fatalf("sandbox manifest must not expose command or shell args: command=%v args=%v", container.Command, container.Args)
	}
	if container.SecurityContext == nil {
		t.Fatal("container security context is required")
	}
	if container.SecurityContext.Privileged != nil && *container.SecurityContext.Privileged {
		t.Fatal("sandbox container must not be privileged")
	}
	if container.SecurityContext.AllowPrivilegeEscalation == nil || *container.SecurityContext.AllowPrivilegeEscalation {
		t.Fatalf("allowPrivilegeEscalation must be false: %+v", container.SecurityContext.AllowPrivilegeEscalation)
	}
	if container.SecurityContext.RunAsNonRoot == nil || !*container.SecurityContext.RunAsNonRoot {
		t.Fatalf("container must run as non-root: %+v", container.SecurityContext.RunAsNonRoot)
	}
	if container.SecurityContext.Capabilities == nil || len(container.SecurityContext.Capabilities.Drop) != 1 || container.SecurityContext.Capabilities.Drop[0] != corev1.Capability("ALL") {
		t.Fatalf("container must drop all capabilities: %+v", container.SecurityContext.Capabilities)
	}
	if container.Resources.Limits.Memory().Value() != req.Policy.MemoryLimitBytes {
		t.Fatalf("memory limit = %d, want %d", container.Resources.Limits.Memory().Value(), req.Policy.MemoryLimitBytes)
	}
	if container.Resources.Limits.Cpu().MilliValue() <= 0 {
		t.Fatalf("cpu limit must be set from policy: %+v", container.Resources.Limits)
	}
	if workload.Job.Spec.ActiveDeadlineSeconds == nil || *workload.Job.Spec.ActiveDeadlineSeconds != int64(req.Policy.Timeout/time.Second) {
		t.Fatalf("active deadline = %v, want %d", workload.Job.Spec.ActiveDeadlineSeconds, int64(req.Policy.Timeout/time.Second))
	}
	assertContainerEnv(t, container, "PYTHON_EXECUTOR_CODE_PATH", "/sandbox/input/main.py")
	assertContainerEnv(t, container, "PYTHON_EXECUTOR_MAX_OUTPUT_BYTES", "65536")
}

func TestKubernetesManifestRejectsUnsupportedOrExcessivePolicy(t *testing.T) {
	tests := []struct {
		name   string
		config KubernetesExecutorConfig
		mutate func(*Request)
		want   string
	}{
		{
			name:   "timeout over executor max",
			config: validKubernetesExecutorConfig(),
			mutate: func(req *Request) {
				req.Policy.Timeout = 31 * time.Second
			},
			want: "timeout",
		},
		{
			name:   "memory over executor max",
			config: validKubernetesExecutorConfig(),
			mutate: func(req *Request) {
				req.Policy.MemoryLimitBytes = 257 * 1024 * 1024
			},
			want: "memory",
		},
		{
			name:   "output over executor max",
			config: validKubernetesExecutorConfig(),
			mutate: func(req *Request) {
				req.Policy.MaxOutputBytes = 65 * 1024
			},
			want: "max_output_bytes",
		},
		{
			name:   "network enabled",
			config: validKubernetesExecutorConfig(),
			mutate: func(req *Request) {
				req.Policy.Network = NetworkPolicyOutboundAllowed
			},
			want: "network",
		},
		{
			name:   "file allowlist unsupported",
			config: validKubernetesExecutorConfig(),
			mutate: func(req *Request) {
				req.Policy.FileAllowlist = []FileAllowlistEntry{{Path: "input.csv", ReadOnly: true}}
			},
			want: "file allowlist materialization is not implemented",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := validKubernetesRequest()
			tt.mutate(&req)

			_, err := buildKubernetesSandboxWorkload(tt.config, req)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected error containing %q, got %v", tt.want, err)
			}
		})
	}
}

func TestKubernetesExecutorLifecycleSuccess(t *testing.T) {
	client := &fakeKubernetesSandboxClient{
		waitResult: KubernetesJobResult{Succeeded: true, ExitCode: 0},
		logResult:  KubernetesLogResult{Stdout: "2\n", ResultJSON: []byte(`{"value":2}`)},
	}
	executor, err := NewKubernetesExecutor(validKubernetesExecutorConfig(), client)
	if err != nil {
		t.Fatalf("new executor: %v", err)
	}

	resp, err := executor.Execute(context.Background(), validKubernetesRequest())
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if resp.Stdout != "2\n" || string(resp.ResultJSON) != `{"value":2}` || resp.ExitCode != 0 || resp.TimedOut {
		t.Fatalf("response mismatch: %+v", resp)
	}
	if client.createdJob == nil || client.createdConfigMap == nil {
		t.Fatal("expected executor to create configmap and job")
	}
	if len(client.deletedJobs) != 1 || len(client.deletedConfigMaps) != 1 {
		t.Fatalf("expected cleanup for job and configmap, got jobs=%v configmaps=%v", client.deletedJobs, client.deletedConfigMaps)
	}
}

func TestKubernetesExecutorLifecycleTimeout(t *testing.T) {
	client := &fakeKubernetesSandboxClient{
		waitResult: KubernetesJobResult{TimedOut: true, ExitCode: -1, Message: "deadline exceeded"},
	}
	executor, err := NewKubernetesExecutor(validKubernetesExecutorConfig(), client)
	if err != nil {
		t.Fatalf("new executor: %v", err)
	}

	resp, err := executor.Execute(context.Background(), validKubernetesRequest())
	if err != nil {
		t.Fatalf("execute timeout should return structured response, got error %v", err)
	}
	if !resp.TimedOut || resp.Error == nil || resp.Error.Code != "timeout" || resp.ExitCode != -1 {
		t.Fatalf("timeout response mismatch: %+v", resp)
	}
	if len(client.deletedJobs) != 1 || len(client.deletedConfigMaps) != 1 {
		t.Fatalf("timeout must cleanup workload, got jobs=%v configmaps=%v", client.deletedJobs, client.deletedConfigMaps)
	}
}

func TestKubernetesExecutorTruncatesOversizedLogs(t *testing.T) {
	cfg := validKubernetesExecutorConfig()
	cfg.MaxOutputBytes = 4
	req := validKubernetesRequest()
	req.Policy.MaxOutputBytes = 4
	client := &fakeKubernetesSandboxClient{
		waitResult: KubernetesJobResult{Succeeded: true, ExitCode: 0},
		logResult:  KubernetesLogResult{Stdout: "abcdef"},
	}
	executor, err := NewKubernetesExecutor(cfg, client)
	if err != nil {
		t.Fatalf("new executor: %v", err)
	}

	resp, err := executor.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if resp.Stdout != "abcd" || !resp.OutputTruncated {
		t.Fatalf("expected truncated stdout, got %+v", resp)
	}
}

func TestKubernetesExecutorCreateFailureIsVisible(t *testing.T) {
	client := &fakeKubernetesSandboxClient{createJobErr: errors.New("apiserver unavailable")}
	executor, err := NewKubernetesExecutor(validKubernetesExecutorConfig(), client)
	if err != nil {
		t.Fatalf("new executor: %v", err)
	}

	resp, err := executor.Execute(context.Background(), validKubernetesRequest())
	if err == nil || !strings.Contains(err.Error(), "create sandbox job") {
		t.Fatalf("expected visible create error, got resp=%+v err=%v", resp, err)
	}
	if len(client.deletedConfigMaps) != 1 {
		t.Fatalf("created configmap should be cleaned after job create failure, got %v", client.deletedConfigMaps)
	}
}

func assertContainerEnv(t *testing.T, container corev1.Container, name string, want string) {
	t.Helper()
	for _, env := range container.Env {
		if env.Name == name {
			if env.Value != want {
				t.Fatalf("env %s = %q, want %q", name, env.Value, want)
			}
			return
		}
	}
	t.Fatalf("missing env %s in %+v", name, container.Env)
}

func validKubernetesExecutorConfig() KubernetesExecutorConfig {
	return KubernetesExecutorConfig{
		Namespace:      "agent-python-sandbox",
		Image:          "ghcr.io/wujunhui99/agents_im/python-sandbox:test",
		MaxTimeout:     30 * time.Second,
		MaxMemoryBytes: 256 * 1024 * 1024,
		MaxOutputBytes: 64 * 1024,
	}
}

func validKubernetesRequest() Request {
	return Request{
		Code:   "print(1 + 1)",
		Policy: validPolicy(),
	}
}

type fakeKubernetesSandboxClient struct {
	createConfigMapErr error
	createJobErr       error
	waitErr            error
	readLogsErr        error
	waitResult         KubernetesJobResult
	logResult          KubernetesLogResult
	createdConfigMap   *corev1.ConfigMap
	createdJob         *batchv1.Job
	deletedConfigMaps  []string
	deletedJobs        []string
}

func (c *fakeKubernetesSandboxClient) CreateConfigMap(ctx context.Context, configMap *corev1.ConfigMap) error {
	if c.createConfigMapErr != nil {
		return c.createConfigMapErr
	}
	c.createdConfigMap = configMap
	return nil
}

func (c *fakeKubernetesSandboxClient) CreateJob(ctx context.Context, job *batchv1.Job) error {
	if c.createJobErr != nil {
		return c.createJobErr
	}
	c.createdJob = job
	return nil
}

func (c *fakeKubernetesSandboxClient) WaitForJob(ctx context.Context, namespace string, jobName string) (KubernetesJobResult, error) {
	if c.waitErr != nil {
		return KubernetesJobResult{}, c.waitErr
	}
	return c.waitResult, nil
}

func (c *fakeKubernetesSandboxClient) ReadJobLogs(ctx context.Context, namespace string, jobName string, maxBytes int64) (KubernetesLogResult, error) {
	if c.readLogsErr != nil {
		return KubernetesLogResult{}, c.readLogsErr
	}
	return c.logResult, nil
}

func (c *fakeKubernetesSandboxClient) DeleteJob(ctx context.Context, namespace string, jobName string) error {
	c.deletedJobs = append(c.deletedJobs, jobName)
	return nil
}

func (c *fakeKubernetesSandboxClient) DeleteConfigMap(ctx context.Context, namespace string, configMapName string) error {
	c.deletedConfigMaps = append(c.deletedConfigMaps, configMapName)
	return nil
}
