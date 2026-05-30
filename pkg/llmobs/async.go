package llmobs

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wujunhui99/agents_im/pkg/observability"
	"github.com/zeromicro/go-zero/core/logx"
)

const (
	defaultAsyncQueueSize     = 1024
	defaultAsyncExportTimeout = langfuseExportTimeout
)

// AsyncSink decouples the foreground agent run from remote export: Observe only
// enqueues (non-blocking), and a background worker performs the synchronous
// export against the wrapped sink using its own context. When the queue is full
// events are dropped and counted (agents_im_llmobs_events_dropped_total) instead
// of blocking the caller, so export-backend latency never leaks into agent runs.
type AsyncSink struct {
	underlying    Sink
	backend       string
	queue         chan Event
	exportTimeout time.Duration
	done          chan struct{}
	wg            sync.WaitGroup
	closeOnce     sync.Once
	dropped       atomic.Int64
}

// NewAsyncSink wraps underlying with a buffered queue and a background export
// worker. queueSize <= 0 uses the default. The worker runs until Close.
func NewAsyncSink(underlying Sink, backend string, queueSize int) *AsyncSink {
	if queueSize <= 0 {
		queueSize = defaultAsyncQueueSize
	}
	backend = strings.TrimSpace(backend)
	if backend == "" {
		backend = BackendNoop
	}
	s := &AsyncSink{
		underlying:    underlying,
		backend:       backend,
		queue:         make(chan Event, queueSize),
		exportTimeout: defaultAsyncExportTimeout,
		done:          make(chan struct{}),
	}
	s.wg.Add(1)
	go s.run()
	return s
}

// Observe enqueues the event for background export and returns immediately. The
// foreground context is intentionally ignored for the export itself (it would be
// cancelled once the request completes); the worker uses its own timeout.
func (s *AsyncSink) Observe(_ context.Context, event Event) (ObserveResult, error) {
	result := ObserveResult{Backend: s.backend, Exported: false}
	select {
	case s.queue <- cloneEvent(event):
	default:
		s.dropped.Add(1)
		observability.RecordLLMObservabilityDrop(s.backend)
		result.DisabledReason = "llm observability queue full; event dropped"
	}
	return result, nil
}

// Dropped reports how many events were dropped due to backpressure.
func (s *AsyncSink) Dropped() int64 { return s.dropped.Load() }

// Close stops accepting new work, drains the queue, and waits for the worker.
// Safe to call multiple times.
func (s *AsyncSink) Close() {
	s.closeOnce.Do(func() { close(s.done) })
	s.wg.Wait()
}

func (s *AsyncSink) run() {
	defer s.wg.Done()
	for {
		select {
		case event := <-s.queue:
			s.export(event)
		case <-s.done:
			for {
				select {
				case event := <-s.queue:
					s.export(event)
				default:
					return
				}
			}
		}
	}
}

func (s *AsyncSink) export(event Event) {
	if s.underlying == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), s.exportTimeout)
	defer cancel()
	if _, err := s.underlying.Observe(ctx, event); err != nil {
		logx.Errorf("llm observability async export failed backend=%q trace_id=%q request_id=%q event_type=%q status=%q: %s",
			s.backend, event.TraceID, event.RequestID, event.Type, event.Status, RedactPlainText(err.Error()))
	}
}
