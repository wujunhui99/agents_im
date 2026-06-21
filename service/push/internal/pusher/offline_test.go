package pusher

import (
	"context"
	"errors"
	"testing"

	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/wujunhui99/agents_im/pkg/messaging"
)

type fakePusher struct {
	calls   [][]string
	channel string
	err     error
}

func (f *fakePusher) Push(_ context.Context, userIDs []string, _ messaging.MessageEvent) error {
	f.calls = append(f.calls, userIDs)
	return f.err
}

func (f *fakePusher) Channel() string {
	if f.channel == "" {
		return "fake"
	}
	return f.channel
}

func TestOfflineHandlerPushesRecipients(t *testing.T) {
	pusher := &fakePusher{}
	handler, err := NewOfflineHandler(pusher)
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}
	record := acceptedRecord(t, []string{"receiver", "receiver2"})
	if err := handler.HandleBatch(context.Background(), []*kgo.Record{record}); err != nil {
		t.Fatalf("handle batch: %v", err)
	}
	if len(pusher.calls) != 1 || len(pusher.calls[0]) != 2 {
		t.Fatalf("expected one push with 2 recipients, got %v", pusher.calls)
	}
}

func TestOfflineHandlerErrorRetriesBatch(t *testing.T) {
	pusher := &fakePusher{err: errors.New("vendor down")}
	handler, _ := NewOfflineHandler(pusher)
	record := acceptedRecord(t, []string{"receiver"})
	if err := handler.HandleBatch(context.Background(), []*kgo.Record{record}); err == nil {
		t.Fatal("expected error so the batch retries")
	}
}

func TestAuditOfflinePusherIsSuccessNoop(t *testing.T) {
	p := NewAuditOfflinePusher()
	if p.Channel() != "audit" {
		t.Fatalf("expected channel audit, got %s", p.Channel())
	}
	if err := p.Push(context.Background(), []string{"u1"}, messaging.MessageEvent{}); err != nil {
		t.Fatalf("audit pusher should not error: %v", err)
	}
}
