package pythonexec

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strings"
	"time"
)

var (
	ErrPythonExecutorDisabled = errors.New("python executor is disabled")
	ErrInvalidRequest         = errors.New("invalid python execution request")
	ErrInvalidPolicy          = errors.New("invalid python execution policy")
)

type Executor interface {
	Execute(ctx context.Context, req Request) (*Response, error)
}

type Request struct {
	Code   string
	Policy Policy
}

func (r Request) Validate() error {
	if strings.TrimSpace(r.Code) == "" {
		return fmt.Errorf("%w: code is required", ErrInvalidRequest)
	}
	if err := r.Policy.Validate(); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidRequest, err)
	}
	return nil
}

type Policy struct {
	RunID            string
	AuditID          string
	Timeout          time.Duration
	CPUTimeLimit     time.Duration
	MemoryLimitBytes int64
	Network          NetworkPolicy
	FileAllowlist    []FileAllowlistEntry
	MaxOutputBytes   int64
}

func (p Policy) Validate() error {
	if strings.TrimSpace(p.RunID) == "" {
		return fmt.Errorf("%w: run_id is required", ErrInvalidPolicy)
	}
	if strings.TrimSpace(p.AuditID) == "" {
		return fmt.Errorf("%w: audit_id is required", ErrInvalidPolicy)
	}
	if p.Timeout <= 0 {
		return fmt.Errorf("%w: timeout must be greater than zero", ErrInvalidPolicy)
	}
	if p.CPUTimeLimit <= 0 {
		return fmt.Errorf("%w: cpu limit must be greater than zero", ErrInvalidPolicy)
	}
	if p.MemoryLimitBytes <= 0 {
		return fmt.Errorf("%w: memory limit must be greater than zero", ErrInvalidPolicy)
	}
	if p.EffectiveNetworkPolicy() != NetworkPolicyDisabled {
		return fmt.Errorf("%w: network must be disabled by default", ErrInvalidPolicy)
	}
	if p.FileAllowlist == nil {
		return fmt.Errorf("%w: file_allowlist must be explicit", ErrInvalidPolicy)
	}
	if p.MaxOutputBytes <= 0 {
		return fmt.Errorf("%w: max_output_bytes must be greater than zero", ErrInvalidPolicy)
	}

	for i, entry := range p.FileAllowlist {
		if err := entry.Validate(); err != nil {
			return fmt.Errorf("%w: file_allowlist[%d]: %v", ErrInvalidPolicy, i, err)
		}
	}
	return nil
}

func (p Policy) EffectiveNetworkPolicy() NetworkPolicy {
	if p.Network == "" {
		return NetworkPolicyDisabled
	}
	return p.Network
}

type NetworkPolicy string

const (
	NetworkPolicyDisabled        NetworkPolicy = "disabled"
	NetworkPolicyOutboundAllowed NetworkPolicy = "outbound_allowed"
)

type FileAllowlistEntry struct {
	Path      string
	ReadOnly  bool
	SHA256    string
	SizeBytes int64
}

func (e FileAllowlistEntry) Validate() error {
	allowPath := strings.TrimSpace(e.Path)
	if allowPath == "" {
		return errors.New("path is required")
	}
	if strings.Contains(allowPath, "\x00") {
		return errors.New("path contains null byte")
	}
	if strings.Contains(allowPath, "\\") {
		return errors.New("path must use slash separators")
	}
	if path.IsAbs(allowPath) {
		return errors.New("path must be relative")
	}

	clean := path.Clean(allowPath)
	if clean != allowPath || clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return errors.New("path must be a clean relative path")
	}
	if !e.ReadOnly {
		return errors.New("read_only must be true")
	}
	if e.SizeBytes < 0 {
		return errors.New("size_bytes must be non-negative")
	}
	return nil
}

type Response struct {
	RunID           string
	AuditID         string
	Stdout          string
	Stderr          string
	ResultJSON      []byte
	ExitCode        int
	TimedOut        bool
	OutputTruncated bool
	Error           *ExecutionError
}

type ExecutionError struct {
	Code    string
	Message string
}

type DisabledExecutor struct{}

func NewDisabledExecutor() *DisabledExecutor {
	return &DisabledExecutor{}
}

func NewDefaultExecutor() Executor {
	return NewDisabledExecutor()
}

func (e *DisabledExecutor) Execute(ctx context.Context, req Request) (*Response, error) {
	if ctx == nil {
		return nil, fmt.Errorf("%w: context is required", ErrInvalidRequest)
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}
	return nil, ErrPythonExecutorDisabled
}
