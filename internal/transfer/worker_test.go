package transfer

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestWorkerConsumesEventAndMarksSuccessful(t *testing.T) {
	consumer := NewInMemoryConsumer(testEnvelope("evt_1"))
	dispatcher := NewInMemoryDispatcher()
	worker := NewWorker(
		consumer,
		dispatcher,
		WithIdempotencyStore(NewMemoryIdempotencyStore()),
		WithPollInterval(time.Millisecond),
	)

	result := worker.RunOnce(context.Background())
	if result.Status != StatusSucceeded {
		t.Fatalf("status = %q, want %q: %v", result.Status, StatusSucceeded, result.Err)
	}
	if len(dispatcher.Calls()) != 1 {
		t.Fatalf("dispatch calls = %d, want 1", len(dispatcher.Calls()))
	}
	if len(consumer.Successful()) != 1 {
		t.Fatalf("successful marks = %d, want 1", len(consumer.Successful()))
	}
}

func TestWorkerIdempotencySkipsDuplicateDispatch(t *testing.T) {
	consumer := NewInMemoryConsumer(testEnvelope("evt_1"), testEnvelope("evt_1"))
	dispatcher := NewInMemoryDispatcher()
	worker := NewWorker(
		consumer,
		dispatcher,
		WithIdempotencyStore(NewMemoryIdempotencyStore()),
		WithPollInterval(time.Millisecond),
	)

	first := worker.RunOnce(context.Background())
	second := worker.RunOnce(context.Background())

	if first.Status != StatusSucceeded || second.Status != StatusSucceeded {
		t.Fatalf("statuses = %q/%q, want succeeded/succeeded", first.Status, second.Status)
	}
	if len(dispatcher.Calls()) != 1 {
		t.Fatalf("dispatch calls = %d, want 1", len(dispatcher.Calls()))
	}
	if len(consumer.Successful()) != 2 {
		t.Fatalf("successful marks = %d, want 2", len(consumer.Successful()))
	}
}

func TestWorkerRetryableFailureDoesNotMarkSuccessful(t *testing.T) {
	consumer := NewInMemoryConsumer(testEnvelope("evt_retry"))
	dispatcher := NewInMemoryDispatcher()
	dispatcher.SetResult(DispatchRetryable(errors.New("gateway unavailable"), 25*time.Millisecond))
	worker := NewWorker(
		consumer,
		dispatcher,
		WithIdempotencyStore(NewMemoryIdempotencyStore()),
		WithPollInterval(time.Millisecond),
		WithRetryBackoff(50*time.Millisecond),
	)

	result := worker.RunOnce(context.Background())
	if result.Status != StatusRetryable || !result.Retryable {
		t.Fatalf("result = %+v, want retryable", result)
	}
	if len(consumer.Successful()) != 0 {
		t.Fatalf("successful marks = %d, want 0", len(consumer.Successful()))
	}
	retries := consumer.Retries()
	if len(retries) != 1 {
		t.Fatalf("retries = %d, want 1", len(retries))
	}
	if retries[0].Decision.Backoff != 25*time.Millisecond {
		t.Fatalf("retry backoff = %s, want 25ms", retries[0].Decision.Backoff)
	}
}

func TestWorkerContextCancellationStopsLoop(t *testing.T) {
	consumer := NewInMemoryConsumer()
	dispatcher := NewInMemoryDispatcher()
	worker := NewWorker(consumer, dispatcher, WithPollInterval(time.Millisecond))

	ctx, cancel := context.WithCancel(context.Background())
	if err := worker.Start(ctx); err != nil {
		t.Fatalf("start worker: %v", err)
	}

	cancel()
	select {
	case <-worker.Done():
	case <-time.After(time.Second):
		t.Fatal("worker did not stop after context cancellation")
	}

	stopCtx, stopCancel := context.WithTimeout(context.Background(), time.Second)
	defer stopCancel()
	if err := worker.Stop(stopCtx); err != nil {
		t.Fatalf("stop worker: %v", err)
	}
}

func testEnvelope(eventID string) Envelope {
	return Envelope{
		ID:      eventID,
		Topic:   "message.accepted.v1",
		Key:     "single:user_a:user_b",
		Attempt: 1,
		Event: MessageEvent{
			EventID:        eventID,
			EventType:      EventTypeMessageAccepted,
			ConversationID: "single:user_a:user_b",
			Seq:            1,
			ServerMsgID:    "msg_" + eventID,
			SenderID:       "user_a",
			ReceiverIDs:    []string{"user_b"},
			CreatedAt:      time.Now().UTC().UnixMilli(),
		},
	}
}
