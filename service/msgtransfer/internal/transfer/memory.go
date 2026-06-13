package transfer

import (
	"context"
	"sync"
)

type RetryRecord struct {
	Envelope Envelope
	Decision RetryDecision
}

type FailureRecord struct {
	Envelope Envelope
	Result   ProcessResult
}

type InMemoryConsumer struct {
	mu         sync.Mutex
	queue      []Envelope
	successful []Envelope
	retries    []RetryRecord
	failures   []FailureRecord
	receiveErr error
	successErr error
	retryErr   error
	failedErr  error
}

func NewInMemoryConsumer(events ...Envelope) *InMemoryConsumer {
	queue := append([]Envelope(nil), events...)
	return &InMemoryConsumer{queue: queue}
}

func (c *InMemoryConsumer) Add(events ...Envelope) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.queue = append(c.queue, events...)
}

func (c *InMemoryConsumer) Receive(ctx context.Context) (Envelope, error) {
	if err := ctx.Err(); err != nil {
		return Envelope{}, err
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.receiveErr != nil {
		return Envelope{}, c.receiveErr
	}
	if len(c.queue) == 0 {
		return Envelope{}, ErrNoEvent
	}
	envelope := c.queue[0]
	c.queue = c.queue[1:]
	return envelope, nil
}

func (c *InMemoryConsumer) MarkSuccessful(ctx context.Context, envelope Envelope) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.successErr != nil {
		return c.successErr
	}
	c.successful = append(c.successful, envelope)
	return nil
}

func (c *InMemoryConsumer) MarkRetry(ctx context.Context, envelope Envelope, decision RetryDecision) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.retryErr != nil {
		return c.retryErr
	}
	c.retries = append(c.retries, RetryRecord{
		Envelope: envelope,
		Decision: decision,
	})
	return nil
}

func (c *InMemoryConsumer) MarkFailed(ctx context.Context, envelope Envelope, result ProcessResult) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.failedErr != nil {
		return c.failedErr
	}
	c.failures = append(c.failures, FailureRecord{
		Envelope: envelope,
		Result:   result,
	})
	return nil
}

func (c *InMemoryConsumer) Successful() []Envelope {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]Envelope(nil), c.successful...)
}

func (c *InMemoryConsumer) Retries() []RetryRecord {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]RetryRecord(nil), c.retries...)
}

func (c *InMemoryConsumer) Failures() []FailureRecord {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]FailureRecord(nil), c.failures...)
}

type InMemoryDispatcher struct {
	mu     sync.Mutex
	calls  []Envelope
	result DispatchResult
}

func NewInMemoryDispatcher() *InMemoryDispatcher {
	return &InMemoryDispatcher{result: DispatchSucceeded()}
}

func (d *InMemoryDispatcher) SetResult(result DispatchResult) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.result = result
}

func (d *InMemoryDispatcher) Dispatch(ctx context.Context, envelope Envelope) DispatchResult {
	if err := ctx.Err(); err != nil {
		return DispatchRetryable(err, 0)
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	d.calls = append(d.calls, envelope)
	return normalizeDispatchResult(d.result)
}

func (d *InMemoryDispatcher) Calls() []Envelope {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]Envelope(nil), d.calls...)
}

type NoopDispatcher struct{}

func (NoopDispatcher) Dispatch(ctx context.Context, envelope Envelope) DispatchResult {
	if err := ctx.Err(); err != nil {
		return DispatchRetryable(err, 0)
	}
	return DispatchSucceeded(envelope.Event.ReceiverIDs...)
}
