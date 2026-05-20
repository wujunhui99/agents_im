package pythonexec

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	kubernetesSandboxContainerName = "python-sandbox"
	kubernetesSandboxCodeKey       = "main.py"
	kubernetesSandboxCodePath      = "/sandbox/input/main.py"
	kubernetesSandboxCodeMountPath = "/sandbox/input"
	kubernetesSandboxPollInterval  = 500 * time.Millisecond
)

type KubernetesExecutorConfig struct {
	Namespace          string
	Image              string
	ServiceAccountName string
	RuntimeClassName   string
	MaxTimeout         time.Duration
	MaxMemoryBytes     int64
	MaxOutputBytes     int64
}

type KubernetesSandboxClient interface {
	CreateConfigMap(ctx context.Context, configMap *corev1.ConfigMap) error
	CreateJob(ctx context.Context, job *batchv1.Job) error
	WaitForJob(ctx context.Context, namespace string, jobName string) (KubernetesJobResult, error)
	ReadJobLogs(ctx context.Context, namespace string, jobName string, maxBytes int64) (KubernetesLogResult, error)
	DeleteJob(ctx context.Context, namespace string, jobName string) error
	DeleteConfigMap(ctx context.Context, namespace string, configMapName string) error
}

type KubernetesJobResult struct {
	Succeeded bool
	Failed    bool
	TimedOut  bool
	ExitCode  int
	Message   string
}

type KubernetesLogResult struct {
	Stdout          string
	Stderr          string
	ResultJSON      []byte
	OutputTruncated bool
	Error           *ExecutionError
}

type KubernetesExecutor struct {
	config KubernetesExecutorConfig
	client KubernetesSandboxClient
}

func NewKubernetesExecutor(config KubernetesExecutorConfig, client KubernetesSandboxClient) (*KubernetesExecutor, error) {
	if err := validateKubernetesExecutorConfig(config); err != nil {
		return nil, err
	}
	if client == nil {
		return nil, fmt.Errorf("%w: kubernetes sandbox client is required", ErrInvalidPolicy)
	}
	return &KubernetesExecutor{config: config, client: client}, nil
}

func (e *KubernetesExecutor) Execute(ctx context.Context, req Request) (*Response, error) {
	if e == nil {
		return nil, fmt.Errorf("%w: kubernetes executor is nil", ErrInvalidRequest)
	}
	if ctx == nil {
		return nil, fmt.Errorf("%w: context is required", ErrInvalidRequest)
	}
	workload, err := buildKubernetesSandboxWorkload(e.config, req)
	if err != nil {
		return nil, err
	}

	namespace := e.config.Namespace
	configMapName := workload.CodeConfigMap.Name
	jobName := workload.Job.Name
	configMapCreated := false
	jobCreated := false
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if jobCreated {
			_ = e.client.DeleteJob(cleanupCtx, namespace, jobName)
		}
		if configMapCreated {
			_ = e.client.DeleteConfigMap(cleanupCtx, namespace, configMapName)
		}
	}()

	if err := e.client.CreateConfigMap(ctx, workload.CodeConfigMap); err != nil {
		return nil, fmt.Errorf("create sandbox code configmap: %w", err)
	}
	configMapCreated = true
	if err := e.client.CreateJob(ctx, workload.Job); err != nil {
		return nil, fmt.Errorf("create sandbox job: %w", err)
	}
	jobCreated = true

	waitCtx, cancel := context.WithTimeout(ctx, req.Policy.Timeout)
	defer cancel()
	jobResult, err := e.client.WaitForJob(waitCtx, namespace, jobName)
	if err != nil {
		return nil, fmt.Errorf("wait for sandbox job: %w", err)
	}

	resp := &Response{
		RunID:    req.Policy.RunID,
		AuditID:  req.Policy.AuditID,
		ExitCode: jobResult.ExitCode,
	}
	if jobResult.TimedOut {
		resp.TimedOut = true
		resp.ExitCode = -1
		resp.Error = &ExecutionError{Code: "timeout", Message: firstNonEmptyString(jobResult.Message, "python execution timed out")}
		return resp, nil
	}

	logs, err := e.client.ReadJobLogs(ctx, namespace, jobName, req.Policy.MaxOutputBytes)
	if err != nil {
		return nil, fmt.Errorf("read sandbox job logs: %w", err)
	}
	stdout, stderr, resultJSON, truncated := enforceOutputLimit(logs.Stdout, logs.Stderr, logs.ResultJSON, req.Policy.MaxOutputBytes)
	resp.Stdout = stdout
	resp.Stderr = stderr
	resp.ResultJSON = resultJSON
	resp.OutputTruncated = logs.OutputTruncated || truncated

	if logs.Error != nil {
		resp.Error = logs.Error
	}
	if jobResult.Failed {
		if resp.ExitCode == 0 {
			resp.ExitCode = 1
		}
		if resp.Error == nil {
			resp.Error = &ExecutionError{Code: "execution_failed", Message: firstNonEmptyString(jobResult.Message, "python execution failed")}
		}
	}
	return resp, nil
}

type kubernetesSandboxWorkload struct {
	CodeConfigMap *corev1.ConfigMap
	Job           *batchv1.Job
}

func buildKubernetesSandboxWorkload(config KubernetesExecutorConfig, req Request) (kubernetesSandboxWorkload, error) {
	if err := validateKubernetesExecutorConfig(config); err != nil {
		return kubernetesSandboxWorkload{}, err
	}
	if err := validateKubernetesPolicy(config, req); err != nil {
		return kubernetesSandboxWorkload{}, err
	}

	jobName := kubernetesDNSLabel("pyexec", req.Policy.RunID)
	configMapName := kubernetesDNSLabel("pyexec-code", req.Policy.RunID)
	labels := map[string]string{
		"app.kubernetes.io/name":       "agents-im-python-executor",
		"app.kubernetes.io/component":  "sandbox",
		"agents-im-python-exec-run-id": safeLabelValue(req.Policy.RunID),
	}
	annotations := map[string]string{
		"agents-im/run-id":   req.Policy.RunID,
		"agents-im/audit-id": req.Policy.AuditID,
	}

	codeConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:        configMapName,
			Namespace:   config.Namespace,
			Labels:      copyStringMap(labels),
			Annotations: copyStringMap(annotations),
		},
		Data: map[string]string{
			kubernetesSandboxCodeKey: req.Code,
		},
	}

	activeDeadlineSeconds := int64((req.Policy.Timeout + time.Second - 1) / time.Second)
	backoffLimit := int32(0)
	ttlSeconds := int32(300)
	automountToken := false
	runAsNonRoot := true
	allowPrivilegeEscalation := false
	privileged := false
	readOnlyRootFilesystem := true
	runAsUser := int64(65532)
	runAsGroup := int64(65532)
	cpuMilli := cpuMilliFromPolicy(req.Policy)
	limits := corev1.ResourceList{
		corev1.ResourceCPU:    *resource.NewMilliQuantity(cpuMilli, resource.DecimalSI),
		corev1.ResourceMemory: *resource.NewQuantity(req.Policy.MemoryLimitBytes, resource.BinarySI),
	}
	tmpSizeLimit := resource.NewQuantity(16*1024*1024, resource.BinarySI)

	podSpec := corev1.PodSpec{
		AutomountServiceAccountToken: &automountToken,
		RestartPolicy:                corev1.RestartPolicyNever,
		SecurityContext: &corev1.PodSecurityContext{
			RunAsNonRoot:   &runAsNonRoot,
			RunAsUser:      &runAsUser,
			RunAsGroup:     &runAsGroup,
			SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
		},
		Containers: []corev1.Container{
			{
				Name:            kubernetesSandboxContainerName,
				Image:           config.Image,
				ImagePullPolicy: corev1.PullIfNotPresent,
				Env: []corev1.EnvVar{
					{Name: "PYTHON_EXECUTOR_CODE_PATH", Value: kubernetesSandboxCodePath},
					{Name: "PYTHON_EXECUTOR_MAX_OUTPUT_BYTES", Value: fmt.Sprintf("%d", req.Policy.MaxOutputBytes)},
					{Name: "PYTHON_EXECUTOR_TIMEOUT_SECONDS", Value: fmt.Sprintf("%d", activeDeadlineSeconds)},
				},
				VolumeMounts: []corev1.VolumeMount{
					{Name: "code", MountPath: kubernetesSandboxCodeMountPath, ReadOnly: true},
					{Name: "tmp", MountPath: "/tmp"},
				},
				SecurityContext: &corev1.SecurityContext{
					RunAsNonRoot:             &runAsNonRoot,
					RunAsUser:                &runAsUser,
					RunAsGroup:               &runAsGroup,
					AllowPrivilegeEscalation: &allowPrivilegeEscalation,
					Privileged:               &privileged,
					ReadOnlyRootFilesystem:   &readOnlyRootFilesystem,
					Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
					SeccompProfile:           &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
				},
				Resources: corev1.ResourceRequirements{
					Limits:   limits,
					Requests: limits,
				},
			},
		},
		Volumes: []corev1.Volume{
			{
				Name: "code",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{Name: configMapName},
						Items: []corev1.KeyToPath{
							{Key: kubernetesSandboxCodeKey, Path: kubernetesSandboxCodeKey},
						},
					},
				},
			},
			{
				Name: "tmp",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{
						Medium:    corev1.StorageMediumMemory,
						SizeLimit: tmpSizeLimit,
					},
				},
			},
		},
	}
	if strings.TrimSpace(config.ServiceAccountName) != "" {
		podSpec.ServiceAccountName = strings.TrimSpace(config.ServiceAccountName)
	}
	if strings.TrimSpace(config.RuntimeClassName) != "" {
		runtimeClassName := strings.TrimSpace(config.RuntimeClassName)
		podSpec.RuntimeClassName = &runtimeClassName
	}

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:        jobName,
			Namespace:   config.Namespace,
			Labels:      copyStringMap(labels),
			Annotations: copyStringMap(annotations),
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            &backoffLimit,
			TTLSecondsAfterFinished: &ttlSeconds,
			ActiveDeadlineSeconds:   &activeDeadlineSeconds,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      copyStringMap(labels),
					Annotations: copyStringMap(annotations),
				},
				Spec: podSpec,
			},
		},
	}
	return kubernetesSandboxWorkload{CodeConfigMap: codeConfigMap, Job: job}, nil
}

func validateKubernetesExecutorConfig(config KubernetesExecutorConfig) error {
	if strings.TrimSpace(config.Namespace) == "" {
		return fmt.Errorf("%w: kubernetes namespace is required", ErrInvalidPolicy)
	}
	if strings.TrimSpace(config.Image) == "" {
		return fmt.Errorf("%w: kubernetes image is required", ErrInvalidPolicy)
	}
	if config.MaxTimeout <= 0 {
		return fmt.Errorf("%w: kubernetes max timeout must be greater than zero", ErrInvalidPolicy)
	}
	if config.MaxMemoryBytes <= 0 {
		return fmt.Errorf("%w: kubernetes max memory must be greater than zero", ErrInvalidPolicy)
	}
	if config.MaxOutputBytes <= 0 {
		return fmt.Errorf("%w: kubernetes max output bytes must be greater than zero", ErrInvalidPolicy)
	}
	return nil
}

func validateKubernetesPolicy(config KubernetesExecutorConfig, req Request) error {
	if err := req.Validate(); err != nil {
		return err
	}
	if len(req.Policy.FileAllowlist) > 0 {
		return fmt.Errorf("%w: file allowlist materialization is not implemented for kubernetes executor", ErrInvalidPolicy)
	}
	if req.Policy.Timeout > config.MaxTimeout {
		return fmt.Errorf("%w: timeout exceeds kubernetes executor max", ErrInvalidPolicy)
	}
	if req.Policy.CPUTimeLimit > req.Policy.Timeout {
		return fmt.Errorf("%w: cpu limit cannot exceed timeout for kubernetes executor", ErrInvalidPolicy)
	}
	if req.Policy.MemoryLimitBytes > config.MaxMemoryBytes {
		return fmt.Errorf("%w: memory exceeds kubernetes executor max", ErrInvalidPolicy)
	}
	if req.Policy.MaxOutputBytes > config.MaxOutputBytes {
		return fmt.Errorf("%w: max_output_bytes exceeds kubernetes executor max", ErrInvalidPolicy)
	}
	return nil
}

func cpuMilliFromPolicy(policy Policy) int64 {
	timeoutMillis := policy.Timeout.Milliseconds()
	if timeoutMillis <= 0 {
		return 100
	}
	cpuMilli := (policy.CPUTimeLimit.Milliseconds()*1000 + timeoutMillis - 1) / timeoutMillis
	if cpuMilli < 50 {
		return 50
	}
	if cpuMilli > 1000 {
		return 1000
	}
	return cpuMilli
}

func kubernetesDNSLabel(prefix string, value string) string {
	normalized := sanitizeDNSLabel(value)
	hash := sha256.Sum256([]byte(value))
	suffix := hex.EncodeToString(hash[:])[:10]
	if normalized == "" {
		normalized = suffix
	}
	maxBase := 63 - len(prefix) - len(suffix) - 2
	if maxBase < 1 {
		maxBase = 1
	}
	if len(normalized) > maxBase {
		normalized = strings.Trim(normalized[:maxBase], "-")
	}
	if normalized == "" {
		normalized = suffix
	}
	return prefix + "-" + normalized + "-" + suffix
}

func sanitizeDNSLabel(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		valid := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if valid {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func safeLabelValue(value string) string {
	label := sanitizeDNSLabel(value)
	if label == "" {
		return "run"
	}
	if len(label) > 63 {
		return strings.Trim(label[:63], "-")
	}
	return label
}

func enforceOutputLimit(stdout string, stderr string, resultJSON []byte, maxBytes int64) (string, string, []byte, bool) {
	if maxBytes <= 0 {
		return "", "", nil, true
	}
	remaining := maxBytes
	truncated := false
	stdout, remaining, truncated = truncateStringBytes(stdout, remaining, truncated)
	stderr, remaining, truncated = truncateStringBytes(stderr, remaining, truncated)
	if int64(len(resultJSON)) > remaining {
		if remaining < 0 {
			remaining = 0
		}
		resultJSON = resultJSON[:remaining]
		truncated = true
	}
	return stdout, stderr, resultJSON, truncated
}

func truncateStringBytes(value string, remaining int64, alreadyTruncated bool) (string, int64, bool) {
	if remaining <= 0 {
		if value != "" {
			return "", 0, true
		}
		return "", 0, alreadyTruncated
	}
	if int64(len(value)) <= remaining {
		return value, remaining - int64(len(value)), alreadyTruncated
	}
	return value[:remaining], 0, true
}

func copyStringMap(values map[string]string) map[string]string {
	copied := make(map[string]string, len(values))
	for key, value := range values {
		copied[key] = value
	}
	return copied
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

type clientGoKubernetesSandboxClient struct {
	client       kubernetes.Interface
	pollInterval time.Duration
}

func NewInClusterKubernetesSandboxClient() (KubernetesSandboxClient, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return NewClientGoKubernetesSandboxClient(client), nil
}

func NewClientGoKubernetesSandboxClient(client kubernetes.Interface) KubernetesSandboxClient {
	return &clientGoKubernetesSandboxClient{client: client, pollInterval: kubernetesSandboxPollInterval}
}

func (c *clientGoKubernetesSandboxClient) CreateConfigMap(ctx context.Context, configMap *corev1.ConfigMap) error {
	_, err := c.client.CoreV1().ConfigMaps(configMap.Namespace).Create(ctx, configMap, metav1.CreateOptions{})
	return err
}

func (c *clientGoKubernetesSandboxClient) CreateJob(ctx context.Context, job *batchv1.Job) error {
	_, err := c.client.BatchV1().Jobs(job.Namespace).Create(ctx, job, metav1.CreateOptions{})
	return err
}

func (c *clientGoKubernetesSandboxClient) WaitForJob(ctx context.Context, namespace string, jobName string) (KubernetesJobResult, error) {
	interval := c.pollInterval
	if interval <= 0 {
		interval = kubernetesSandboxPollInterval
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		result, done, err := c.inspectJob(ctx, namespace, jobName)
		if err != nil {
			return KubernetesJobResult{}, err
		}
		if done {
			return result, nil
		}
		select {
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return KubernetesJobResult{TimedOut: true, ExitCode: -1, Message: ctx.Err().Error()}, nil
			}
			return KubernetesJobResult{}, ctx.Err()
		case <-ticker.C:
		}
	}
}

func (c *clientGoKubernetesSandboxClient) inspectJob(ctx context.Context, namespace string, jobName string) (KubernetesJobResult, bool, error) {
	job, err := c.client.BatchV1().Jobs(namespace).Get(ctx, jobName, metav1.GetOptions{})
	if err != nil {
		return KubernetesJobResult{}, false, err
	}
	if job.Status.Succeeded > 0 {
		return KubernetesJobResult{Succeeded: true, ExitCode: 0}, true, nil
	}
	if job.Status.Failed > 0 {
		exitCode := c.findJobExitCode(ctx, namespace, jobName)
		if exitCode == 0 {
			exitCode = 1
		}
		return KubernetesJobResult{Failed: true, ExitCode: exitCode, Message: jobConditionMessage(job.Status.Conditions)}, true, nil
	}
	for _, condition := range job.Status.Conditions {
		if condition.Type == batchv1.JobFailed && condition.Reason == "DeadlineExceeded" {
			return KubernetesJobResult{TimedOut: true, ExitCode: -1, Message: condition.Message}, true, nil
		}
	}
	return KubernetesJobResult{}, false, nil
}

func (c *clientGoKubernetesSandboxClient) ReadJobLogs(ctx context.Context, namespace string, jobName string, maxBytes int64) (KubernetesLogResult, error) {
	pod, err := c.findJobPod(ctx, namespace, jobName)
	if err != nil {
		return KubernetesLogResult{}, err
	}
	stream, err := c.client.CoreV1().Pods(namespace).GetLogs(pod.Name, &corev1.PodLogOptions{
		Container: kubernetesSandboxContainerName,
	}).Stream(ctx)
	if err != nil {
		return KubernetesLogResult{}, err
	}
	defer stream.Close()

	limit := maxBytes + 1
	if limit <= 1 {
		limit = 1
	}
	raw, err := io.ReadAll(io.LimitReader(stream, limit))
	if err != nil {
		return KubernetesLogResult{}, err
	}
	truncated := int64(len(raw)) > maxBytes
	if truncated && maxBytes >= 0 {
		raw = raw[:maxBytes]
	}
	return parseKubernetesLogResult(raw, truncated), nil
}

func (c *clientGoKubernetesSandboxClient) DeleteJob(ctx context.Context, namespace string, jobName string) error {
	propagation := metav1.DeletePropagationBackground
	err := c.client.BatchV1().Jobs(namespace).Delete(ctx, jobName, metav1.DeleteOptions{PropagationPolicy: &propagation})
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

func (c *clientGoKubernetesSandboxClient) DeleteConfigMap(ctx context.Context, namespace string, configMapName string) error {
	err := c.client.CoreV1().ConfigMaps(namespace).Delete(ctx, configMapName, metav1.DeleteOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

func (c *clientGoKubernetesSandboxClient) findJobExitCode(ctx context.Context, namespace string, jobName string) int {
	pod, err := c.findJobPod(ctx, namespace, jobName)
	if err != nil {
		return 0
	}
	for _, status := range pod.Status.ContainerStatuses {
		if status.Name != kubernetesSandboxContainerName || status.State.Terminated == nil {
			continue
		}
		return int(status.State.Terminated.ExitCode)
	}
	return 0
}

func (c *clientGoKubernetesSandboxClient) findJobPod(ctx context.Context, namespace string, jobName string) (*corev1.Pod, error) {
	selector := labels.SelectorFromSet(labels.Set{"job-name": jobName}).String()
	pods, err := c.client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return nil, err
	}
	if len(pods.Items) == 0 {
		return nil, fmt.Errorf("no pod found for sandbox job %q", jobName)
	}
	return &pods.Items[0], nil
}

func jobConditionMessage(conditions []batchv1.JobCondition) string {
	for _, condition := range conditions {
		if strings.TrimSpace(condition.Message) != "" {
			return condition.Message
		}
	}
	return ""
}

func parseKubernetesLogResult(raw []byte, truncated bool) KubernetesLogResult {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return KubernetesLogResult{OutputTruncated: truncated}
	}
	var result struct {
		Stdout          string          `json:"stdout"`
		Stderr          string          `json:"stderr"`
		ResultJSON      json.RawMessage `json:"result_json"`
		ExitCode        int             `json:"exit_code"`
		TimedOut        bool            `json:"timed_out"`
		OutputTruncated bool            `json:"output_truncated"`
		Error           *ExecutionError `json:"error"`
	}
	if err := json.Unmarshal(raw, &result); err == nil {
		return KubernetesLogResult{
			Stdout:          result.Stdout,
			Stderr:          result.Stderr,
			ResultJSON:      []byte(result.ResultJSON),
			OutputTruncated: truncated || result.OutputTruncated,
			Error:           result.Error,
		}
	}
	return KubernetesLogResult{Stdout: string(raw), OutputTruncated: truncated}
}
