package llmobs

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
)

func TestAsyncSinkExportsInBackground(t *testing.T) {
	var got int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&got, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"successes":[{"id":"accepted","status":201}],"errors":[]}`))
	}))
	defer server.Close()

	underlying, err := newLangfuseSink(Config{
		Enabled: true,
		Backend: BackendLangfuse,
		Langfuse: LangfuseConfig{
			Host:      server.URL,
			PublicKey: "pk-lf-unit-test",
			SecretKey: "sk-lf-unit-test",
		},
	})
	if err != nil {
		t.Fatalf("new langfuse sink: %v", err)
	}
	sink := NewAsyncSink(underlying, BackendLangfuse, 8)

	result, err := sink.Observe(context.Background(), sampleLangfuseEvent())
	if err != nil {
		t.Fatalf("async observe: %v", err)
	}
	if result.Exported {
		t.Fatalf("async observe must not report synchronous export: %+v", result)
	}

	sink.Close() // drains queue + waits for the worker

	if atomic.LoadInt32(&got) == 0 {
		t.Fatalf("background worker did not export to langfuse")
	}
	if sink.Dropped() != 0 {
		t.Fatalf("unexpected drops: %d", sink.Dropped())
	}
}

func TestAsyncSinkDropsOnBackpressure(t *testing.T) {
	release := make(chan struct{})
	sink := NewAsyncSink(&blockingSink{release: release}, BackendLangfuse, 1)

	// The worker grabs one event and blocks in export; the queue (cap 1) holds one
	// more; the rest are dropped rather than blocking the caller.
	for i := 0; i < 200; i++ {
		if _, err := sink.Observe(context.Background(), sampleLangfuseEvent()); err != nil {
			t.Fatalf("observe %d: %v", i, err)
		}
	}
	if sink.Dropped() == 0 {
		t.Fatalf("expected drops under backpressure, got 0")
	}

	close(release)
	sink.Close()
}

type blockingSink struct {
	release chan struct{}
	mu      sync.Mutex
	calls   int
}

func (s *blockingSink) Observe(ctx context.Context, _ Event) (ObserveResult, error) {
	s.mu.Lock()
	s.calls++
	s.mu.Unlock()
	select {
	case <-s.release:
	case <-ctx.Done():
	}
	return ObserveResult{Backend: BackendLangfuse, Exported: true}, nil
}
