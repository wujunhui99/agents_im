package transfer

import (
	"context"
	"errors"
	"sync"
	"time"
)

const (
	defaultPollInterval = 100 * time.Millisecond
	defaultRetryBackoff = time.Second
	defaultMaxAttempts  = 5
)

type Worker struct {
	consumer     EventConsumer
	dispatcher   DeliveryDispatcher
	idempotency  IdempotencyStore
	recorder     DeliveryAttemptRecorder
	workerID     string
	pollInterval time.Duration
	retryBackoff time.Duration
	maxAttempts  int

	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
}

type WorkerOption func(*Worker)

func NewWorker(consumer EventConsumer, dispatcher DeliveryDispatcher, opts ...WorkerOption) *Worker {
	w := &Worker{
		consumer:     consumer,
		dispatcher:   dispatcher,
		idempotency:  NoopIdempotencyStore{},
		pollInterval: defaultPollInterval,
		retryBackoff: defaultRetryBackoff,
		maxAttempts:  defaultMaxAttempts,
		done:         closedDone(),
	}
	for _, opt := range opts {
		opt(w)
	}
	return w
}

func WithWorkerID(workerID string) WorkerOption {
	return func(w *Worker) {
		w.workerID = workerID
	}
}

func WithIdempotencyStore(store IdempotencyStore) WorkerOption {
	return func(w *Worker) {
		if store != nil {
			w.idempotency = store
		}
	}
}

func WithDeliveryAttemptRecorder(recorder DeliveryAttemptRecorder) WorkerOption {
	return func(w *Worker) {
		w.recorder = recorder
	}
}

func WithPollInterval(interval time.Duration) WorkerOption {
	return func(w *Worker) {
		if interval > 0 {
			w.pollInterval = interval
		}
	}
}

func WithRetryBackoff(backoff time.Duration) WorkerOption {
	return func(w *Worker) {
		if backoff > 0 {
			w.retryBackoff = backoff
		}
	}
}

func WithMaxAttempts(maxAttempts int) WorkerOption {
	return func(w *Worker) {
		if maxAttempts > 0 {
			w.maxAttempts = maxAttempts
		}
	}
}

func (w *Worker) Start(ctx context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.cancel != nil {
		return errors.New("message transfer worker already started")
	}

	runCtx, cancel := context.WithCancel(ctx)
	w.cancel = cancel
	w.done = make(chan struct{})
	go func() {
		defer close(w.done)
		_ = w.Run(runCtx)
	}()
	return nil
}

func (w *Worker) Stop(ctx context.Context) error {
	w.mu.Lock()
	cancel := w.cancel
	done := w.done
	w.cancel = nil
	w.mu.Unlock()

	if cancel == nil {
		return nil
	}
	cancel()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (w *Worker) Done() <-chan struct{} {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.done
}

func (w *Worker) Run(ctx context.Context) error {
	for {
		result := w.RunOnce(ctx)
		switch result.Status {
		case StatusStopped:
			return nil
		case StatusNoEvent:
			if !sleepContext(ctx, w.pollInterval) {
				return nil
			}
		case StatusRetryable:
			delay := result.RetryAfter
			if delay <= 0 {
				delay = w.retryBackoff
			}
			if !sleepContext(ctx, delay) {
				return nil
			}
		default:
		}
	}
}

func (w *Worker) RunOnce(ctx context.Context) ProcessResult {
	if err := ctx.Err(); err != nil {
		return ProcessResult{Status: StatusStopped, Err: err}
	}

	envelope, err := w.consumer.Receive(ctx)
	if err != nil {
		if errors.Is(err, ErrNoEvent) {
			return ProcessResult{Status: StatusNoEvent}
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return ProcessResult{Status: StatusStopped, Err: err}
		}
		return ProcessResult{Status: StatusRetryable, Retryable: true, RetryAfter: w.retryBackoff, Err: err}
	}
	if envelope.Attempt <= 0 {
		envelope.Attempt = 1
	}

	key := envelope.IdempotencyKey()
	base := ProcessResult{
		EventID:        envelope.Event.EventID,
		IdempotencyKey: key,
		Attempt:        envelope.Attempt,
		MaxAttempts:    w.maxAttempts,
	}

	processed, err := w.idempotency.HasProcessed(ctx, key)
	if err != nil {
		result := base
		result.Status = StatusRetryable
		result.Retryable = true
		result.RetryAfter = w.retryBackoff
		result.Err = err
		w.markRetry(ctx, envelope, result)
		return result
	}
	if processed {
		result := base
		result.Status = StatusSucceeded
		result.DeliveredUserIDs = append([]string(nil), envelope.Event.ReceiverIDs...)
		if err := w.consumer.MarkSuccessful(ctx, envelope); err != nil {
			result.Status = StatusRetryable
			result.Retryable = true
			result.RetryAfter = w.retryBackoff
			result.Err = err
		}
		return result
	}

	dispatch := normalizeDispatchResult(w.dispatcher.Dispatch(ctx, envelope))
	result := base
	result.Status = dispatch.Status
	result.Retryable = dispatch.Retryable
	result.RetryAfter = dispatch.RetryAfter
	result.DeliveredUserIDs = append([]string(nil), dispatch.DeliveredUserIDs...)
	result.RecipientResults = cloneRecipientDeliveryResults(dispatch.RecipientResults)
	result.Err = dispatch.Err

	switch result.Status {
	case StatusSucceeded:
		if err := w.recordDeliveryAttempts(ctx, envelope, result); err != nil {
			result.Status = StatusRetryable
			result.Retryable = true
			result.RetryAfter = w.retryBackoff
			result.Err = err
			w.markRetry(ctx, envelope, result)
			return result
		}
		if err := w.idempotency.MarkProcessed(ctx, key); err != nil {
			result.Status = StatusRetryable
			result.Retryable = true
			result.RetryAfter = w.retryBackoff
			result.Err = err
			w.markRetry(ctx, envelope, result)
			return result
		}
		if err := w.consumer.MarkSuccessful(ctx, envelope); err != nil {
			result.Status = StatusRetryable
			result.Retryable = true
			result.RetryAfter = w.retryBackoff
			result.Err = err
			return result
		}
		return result
	case StatusRetryable:
		if result.RetryAfter <= 0 {
			result.RetryAfter = w.retryBackoff
		}
		if w.maxAttempts > 0 && envelope.Attempt >= w.maxAttempts {
			result.Status = StatusFailed
			result.Retryable = false
			result.RetryAfter = 0
			if err := w.recordDeliveryAttempts(ctx, envelope, result); err != nil {
				result.Status = StatusRetryable
				result.Retryable = true
				result.RetryAfter = w.retryBackoff
				result.Err = err
				w.markRetry(ctx, envelope, result)
				return result
			}
			_ = w.consumer.MarkFailed(ctx, envelope, result)
			return result
		}
		if err := w.recordDeliveryAttempts(ctx, envelope, result); err != nil {
			result.Err = err
			w.markRetry(ctx, envelope, result)
			return result
		}
		w.markRetry(ctx, envelope, result)
		return result
	default:
		result.Status = StatusFailed
		result.Retryable = false
		result.RetryAfter = 0
		if err := w.recordDeliveryAttempts(ctx, envelope, result); err != nil {
			result.Status = StatusRetryable
			result.Retryable = true
			result.RetryAfter = w.retryBackoff
			result.Err = err
			w.markRetry(ctx, envelope, result)
			return result
		}
		_ = w.consumer.MarkFailed(ctx, envelope, result)
		return result
	}
}

func (w *Worker) recordDeliveryAttempts(ctx context.Context, envelope Envelope, result ProcessResult) error {
	if w.recorder == nil || len(result.RecipientResults) == 0 {
		return nil
	}
	return w.recorder.RecordDeliveryResults(ctx, envelope, result)
}

func (w *Worker) markRetry(ctx context.Context, envelope Envelope, result ProcessResult) {
	decision := RetryDecision{
		Attempt:       envelope.Attempt,
		MaxAttempts:   w.maxAttempts,
		Backoff:       result.RetryAfter,
		NextAttemptAt: time.Now().UTC().Add(result.RetryAfter),
	}
	if result.Err != nil {
		decision.Reason = result.Err.Error()
	}
	_ = w.consumer.MarkRetry(ctx, envelope, decision)
}

func normalizeDispatchResult(result DispatchResult) DispatchResult {
	if result.Status == "" {
		result.Status = StatusSucceeded
	}
	if result.Status == StatusRetryable {
		result.Retryable = true
	}
	return result
}

func sleepContext(ctx context.Context, duration time.Duration) bool {
	if duration <= 0 {
		return ctx.Err() == nil
	}
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func closedDone() chan struct{} {
	done := make(chan struct{})
	close(done)
	return done
}
