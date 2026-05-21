package llmobs

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
)

var (
	ErrLangfuseConfigMissing = errors.New("langfuse config is missing: set LANGFUSE_HOST, LANGFUSE_PUBLIC_KEY, and LANGFUSE_SECRET_KEY")
)

const defaultMaxOutputBytes = 2048

type Sink interface {
	Observe(ctx context.Context, event Event) (ObserveResult, error)
}

type NoopSink struct {
	backend string
	reason  string
}

func NewNoopSink(reason string) NoopSink {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "llm observability disabled"
	}
	return NoopSink{backend: BackendNoop, reason: reason}
}

func (s NoopSink) Observe(context.Context, Event) (ObserveResult, error) {
	backend := strings.TrimSpace(s.backend)
	if backend == "" {
		backend = BackendNoop
	}
	reason := strings.TrimSpace(s.reason)
	if reason == "" {
		reason = "llm observability disabled"
	}
	return ObserveResult{
		Backend:        backend,
		Exported:       false,
		DisabledReason: reason,
	}, nil
}

type MemorySink struct {
	mu     sync.Mutex
	events []Event
}

func NewMemorySink() *MemorySink {
	return &MemorySink{}
}

func (s *MemorySink) Observe(_ context.Context, event Event) (ObserveResult, error) {
	if s == nil {
		return ObserveResult{Backend: BackendMemory, Exported: false, DisabledReason: "memory sink not configured"}, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, cloneEvent(event))
	return ObserveResult{Backend: BackendMemory, Exported: false}, nil
}

func (s *MemorySink) Events() []Event {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	events := make([]Event, 0, len(s.events))
	for _, event := range s.events {
		events = append(events, cloneEvent(event))
	}
	return events
}

func NewSink(config Config) (Sink, error) {
	config = normalizeConfig(config)
	switch config.Backend {
	case BackendNoop:
		return NewNoopSink("llm observability disabled"), nil
	case BackendMemory, BackendTest:
		return NewMemorySink(), nil
	case BackendLangfuse:
		if !config.Enabled {
			return NoopSink{backend: BackendLangfuse, reason: "langfuse export disabled"}, nil
		}
		if err := validateLangfuseConfig(config.Langfuse); err != nil {
			return nil, err
		}
		return newLangfuseSink(config)
	default:
		return nil, fmt.Errorf("unsupported llm observability backend %q", config.Backend)
	}
}

func normalizeConfig(config Config) Config {
	config.Backend = strings.ToLower(strings.TrimSpace(config.Backend))
	if config.Backend == "" {
		config.Backend = BackendNoop
	}
	config.Langfuse.Host = strings.TrimSpace(config.Langfuse.Host)
	config.Langfuse.PublicKey = strings.TrimSpace(config.Langfuse.PublicKey)
	config.Langfuse.SecretKey = strings.TrimSpace(config.Langfuse.SecretKey)
	if config.MaxOutputBytes < 0 {
		config.MaxOutputBytes = 0
	}
	if config.MaxOutputBytes == 0 {
		config.MaxOutputBytes = defaultMaxOutputBytes
	}
	return config
}

func validateLangfuseConfig(config LangfuseConfig) error {
	if strings.TrimSpace(config.Host) == "" ||
		strings.TrimSpace(config.PublicKey) == "" ||
		strings.TrimSpace(config.SecretKey) == "" ||
		isPlaceholder(config.PublicKey) ||
		isPlaceholder(config.SecretKey) {
		return ErrLangfuseConfigMissing
	}
	return nil
}

func isPlaceholder(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	return value == "" ||
		strings.Contains(value, "placeholder") ||
		strings.Contains(value, "replace-with") ||
		strings.HasPrefix(value, "your-")
}

func cloneEvent(event Event) Event {
	if len(event.Metadata) > 0 {
		metadata := make(map[string]string, len(event.Metadata))
		for key, value := range event.Metadata {
			metadata[key] = value
		}
		event.Metadata = metadata
	}
	return event
}
