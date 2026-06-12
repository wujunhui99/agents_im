package transfergateway

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	gatewaydelivery "github.com/wujunhui99/agents_im/common/share/gateway/delivery"
	"github.com/wujunhui99/agents_im/internal/transfer"
)

func TestDispatcherDeliversMessageAcceptedToGateway(t *testing.T) {
	fake := newFakeGatewayDispatcher(deliveredResult("single:user_a:user_b", "user_b"))
	dispatcher := NewDispatcher(fake)

	result := dispatcher.Dispatch(context.Background(), testEnvelope("evt_success"))
	if result.Status != transfer.StatusSucceeded {
		t.Fatalf("status = %q, want %q: %v", result.Status, transfer.StatusSucceeded, result.Err)
	}
	if len(result.DeliveredUserIDs) != 1 || result.DeliveredUserIDs[0] != "user_b" {
		t.Fatalf("delivered users = %+v, want [user_b]", result.DeliveredUserIDs)
	}

	calls := fake.Calls()
	if len(calls) != 1 {
		t.Fatalf("gateway calls = %d, want 1", len(calls))
	}
	call := calls[0]
	if call.conversationID != "single:user_a:user_b" {
		t.Fatalf("conversation id = %q", call.conversationID)
	}
	if len(call.recipientUserIDs) != 1 || call.recipientUserIDs[0] != "user_b" {
		t.Fatalf("recipients = %+v, want [user_b]", call.recipientUserIDs)
	}
	if call.event.Type != gatewaydelivery.EventMessageReceived {
		t.Fatalf("event type = %q, want %q", call.event.Type, gatewaydelivery.EventMessageReceived)
	}
	if call.event.Data.ServerMsgID != "msg_evt_success" || call.event.Data.Seq != 7 || call.event.Data.ReceiverID != "user_b" {
		t.Fatalf("message identity mismatch: %+v", call.event.Data)
	}
	if call.event.Data.ClientMsgID != "client_evt_success" || call.event.Data.ContentType != "text" || call.event.Data.Content != "hello evt_success" {
		t.Fatalf("message payload mismatch: %+v", call.event.Data)
	}
	if call.event.Data.ContentMetadata["encoding"] != "plain" || call.event.Data.TraceID != "trace_evt_success" {
		t.Fatalf("message metadata mismatch: %+v", call.event.Data)
	}
}

func TestDispatcherOfflineRecipientsAreCompletedWithoutDeliveredUsers(t *testing.T) {
	fake := newFakeGatewayDispatcher(offlineResult("single:user_a:user_b", "user_b"))
	dispatcher := NewDispatcher(fake)

	result := dispatcher.Dispatch(context.Background(), testEnvelope("evt_offline"))
	if result.Status != transfer.StatusSucceeded {
		t.Fatalf("status = %q, want %q: %v", result.Status, transfer.StatusSucceeded, result.Err)
	}
	if len(result.DeliveredUserIDs) != 0 {
		t.Fatalf("delivered users = %+v, want none for offline recipient", result.DeliveredUserIDs)
	}
	if len(fake.Calls()) != 1 {
		t.Fatalf("gateway calls = %d, want 1", len(fake.Calls()))
	}
}

func TestDispatcherNoRecipientsFailsWithoutCallingGateway(t *testing.T) {
	fake := newFakeGatewayDispatcher(deliveredResult("single:user_a:user_b", "user_b"))
	dispatcher := NewDispatcher(fake)
	envelope := testEnvelope("evt_no_recipients")
	envelope.Event.ReceiverID = ""
	envelope.Event.ReceiverIDs = nil

	result := dispatcher.Dispatch(context.Background(), envelope)
	if result.Status != transfer.StatusFailed || result.Retryable {
		t.Fatalf("result = %+v, want terminal failed", result)
	}
	if !errors.Is(result.Err, ErrNoRecipients) {
		t.Fatalf("error = %v, want ErrNoRecipients", result.Err)
	}
	if len(fake.Calls()) != 0 {
		t.Fatalf("gateway calls = %d, want 0", len(fake.Calls()))
	}
}

func TestWorkerIdempotencySkipsDuplicateGatewayDispatch(t *testing.T) {
	envelope := testEnvelope("evt_duplicate")
	consumer := transfer.NewInMemoryConsumer(envelope, envelope)
	fake := newFakeGatewayDispatcher(deliveredResult(envelope.Event.ConversationID, "user_b"))
	worker := transfer.NewWorker(
		consumer,
		NewDispatcher(fake),
		transfer.WithIdempotencyStore(transfer.NewMemoryIdempotencyStore()),
		transfer.WithRetryBackoff(time.Millisecond),
	)

	first := worker.RunOnce(context.Background())
	second := worker.RunOnce(context.Background())
	if first.Status != transfer.StatusSucceeded || second.Status != transfer.StatusSucceeded {
		t.Fatalf("statuses = %q/%q, want succeeded/succeeded", first.Status, second.Status)
	}
	if len(fake.Calls()) != 1 {
		t.Fatalf("gateway calls = %d, want 1", len(fake.Calls()))
	}
	if len(consumer.Successful()) != 2 {
		t.Fatalf("successful marks = %d, want 2", len(consumer.Successful()))
	}
}

func TestWorkerRetryDecisionForGatewayError(t *testing.T) {
	envelope := testEnvelope("evt_retry")
	envelope.Attempt = 2
	consumer := transfer.NewInMemoryConsumer(envelope)
	fake := newFakeGatewayDispatcher(gatewaydelivery.Result{})
	fake.SetError(errors.New("gateway dispatcher offline"))
	worker := transfer.NewWorker(
		consumer,
		NewDispatcher(fake, WithRetryAfter(25*time.Millisecond)),
		transfer.WithIdempotencyStore(transfer.NewMemoryIdempotencyStore()),
		transfer.WithRetryBackoff(100*time.Millisecond),
		transfer.WithMaxAttempts(4),
	)

	result := worker.RunOnce(context.Background())
	if result.Status != transfer.StatusRetryable || !result.Retryable {
		t.Fatalf("result = %+v, want retryable", result)
	}

	retries := consumer.Retries()
	if len(retries) != 1 {
		t.Fatalf("retry records = %d, want 1", len(retries))
	}
	decision := retries[0].Decision
	if decision.Attempt != 2 || decision.MaxAttempts != 4 || decision.Backoff != 25*time.Millisecond {
		t.Fatalf("retry decision mismatch: %+v", decision)
	}
	if !strings.Contains(decision.Reason, "gateway dispatcher offline") {
		t.Fatalf("retry reason = %q, want gateway error", decision.Reason)
	}
}

func TestWorkerRecordsLiveDeliveryMetricsOnly(t *testing.T) {
	envelope := testEnvelope("evt_record_metrics_only")
	consumer := transfer.NewInMemoryConsumer(envelope)
	fake := newFakeGatewayDispatcher(deliveredResult(envelope.Event.ConversationID, "user_b"))
	worker := transfer.NewWorker(
		consumer,
		NewDispatcher(fake),
		transfer.WithIdempotencyStore(transfer.NewMemoryIdempotencyStore()),
		transfer.WithDeliveryAttemptRecorder(transfer.NewMetricsDeliveryAttemptRecorder()),
	)

	result := worker.RunOnce(context.Background())
	if result.Status != transfer.StatusSucceeded {
		t.Fatalf("result = %+v, want succeeded", result)
	}
}

type fakeGatewayDispatcher struct {
	mu     sync.Mutex
	calls  []gatewayCall
	result gatewaydelivery.Result
	err    error
}

type gatewayCall struct {
	conversationID   string
	recipientUserIDs []string
	event            gatewaydelivery.Event
}

func newFakeGatewayDispatcher(result gatewaydelivery.Result) *fakeGatewayDispatcher {
	return &fakeGatewayDispatcher{result: result}
}

func (d *fakeGatewayDispatcher) SetError(err error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.err = err
}

func (d *fakeGatewayDispatcher) DeliverToUser(ctx context.Context, userID string, event gatewaydelivery.Event) (gatewaydelivery.Result, error) {
	return d.DeliverToConversation(ctx, event.Data.ConversationID, []string{userID}, event)
}

func (d *fakeGatewayDispatcher) DeliverToConversation(ctx context.Context, conversationID string, recipientUserIDs []string, event gatewaydelivery.Event) (gatewaydelivery.Result, error) {
	if err := ctx.Err(); err != nil {
		return gatewaydelivery.Result{}, err
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	d.calls = append(d.calls, gatewayCall{
		conversationID:   conversationID,
		recipientUserIDs: append([]string(nil), recipientUserIDs...),
		event:            event,
	})
	return d.result, d.err
}

func (d *fakeGatewayDispatcher) Calls() []gatewayCall {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]gatewayCall(nil), d.calls...)
}

func deliveredResult(conversationID string, userIDs ...string) gatewaydelivery.Result {
	result := gatewaydelivery.Result{ConversationID: conversationID}
	for _, userID := range userIDs {
		result.AddRecipient(gatewaydelivery.RecipientResult{
			UserID:                 userID,
			Status:                 gatewaydelivery.StatusDelivered,
			DeliveredConnectionIDs: []string{"conn_" + userID},
		})
	}
	return result
}

func offlineResult(conversationID string, userIDs ...string) gatewaydelivery.Result {
	result := gatewaydelivery.Result{ConversationID: conversationID}
	for _, userID := range userIDs {
		result.AddRecipient(gatewaydelivery.RecipientResult{
			UserID: userID,
			Status: gatewaydelivery.StatusOffline,
		})
	}
	return result
}

func testEnvelope(eventID string) transfer.Envelope {
	return transfer.Envelope{
		ID:      eventID,
		Topic:   "message.accepted.v1",
		Key:     "single:user_a:user_b",
		Attempt: 1,
		Event: transfer.MessageEvent{
			EventID:        eventID,
			EventType:      transfer.EventTypeMessageAccepted,
			ConversationID: "single:user_a:user_b",
			Seq:            7,
			ServerMsgID:    "msg_" + eventID,
			SenderID:       "user_a",
			ReceiverIDs:    []string{"user_b"},
			ChatType:       "single",
			ClientMsgID:    "client_" + eventID,
			ContentType:    "text",
			Content:        "hello " + eventID,
			ContentMetadata: map[string]interface{}{
				"encoding": "plain",
			},
			SendTime:  1710000000000,
			CreatedAt: 1710000000000,
			TraceID:   "trace_" + eventID,
		},
	}
}
