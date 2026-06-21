package pusher

import (
	"context"
	"fmt"

	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/zeromicro/go-zero/core/logx"

	"github.com/wujunhui99/agents_im/pkg/messaging"
	"github.com/wujunhui99/agents_im/pkg/observability"
)

// OfflinePusher delivers a message to recipients that are offline, via a vendor
// channel (FCM/APNs/Getui/JPush). The first version is audit-only (§6.3): the
// real channels land in a follow-up. It MUST NOT silently swallow a transient
// vendor error — return it so the batch retries.
type OfflinePusher interface {
	// Push delivers event to userIDs. Channel returns the vendor channel label
	// for metrics.
	Push(ctx context.Context, userIDs []string, event messaging.MessageEvent) error
	Channel() string
}

// AuditOfflinePusher is the first-version pusher (§6.3): no vendor integration
// yet, it records an audit log + metric so the offline fan-out is observable end
// to end and a real channel can be slotted in without touching the consumer.
// This is an explicit, documented placeholder (not a fake success masquerading
// as real delivery) per AGENTS.md 必须遵守 §1.
type AuditOfflinePusher struct{}

func NewAuditOfflinePusher() AuditOfflinePusher { return AuditOfflinePusher{} }

func (AuditOfflinePusher) Channel() string { return "audit" }

func (AuditOfflinePusher) Push(ctx context.Context, userIDs []string, event messaging.MessageEvent) error {
	logx.WithContext(ctx).Infow("offline push (audit-only; no vendor channel yet)",
		logx.Field("server_msg_id", event.ServerMsgID),
		logx.Field("conversation_id", event.ConversationID),
		logx.Field("recipients", len(userIDs)),
	)
	return nil
}

// OfflineHandler consumes msg.toOfflinePush.v1 (03 §6.3): drive the vendor pusher
// for each event's offline recipients. At-least-once batch semantics: a pusher
// error returns non-nil so the batch retries; malformed records are logged + skipped.
type OfflineHandler struct {
	pusher OfflinePusher
}

func NewOfflineHandler(pusher OfflinePusher) (*OfflineHandler, error) {
	if pusher == nil {
		return nil, fmt.Errorf("offline handler requires a pusher")
	}
	return &OfflineHandler{pusher: pusher}, nil
}

func (h *OfflineHandler) HandleBatch(ctx context.Context, records []*kgo.Record) error {
	for _, record := range records {
		if err := h.handleOne(ctx, record); err != nil {
			return err
		}
	}
	return nil
}

func (h *OfflineHandler) handleOne(ctx context.Context, record *kgo.Record) error {
	event, err := messaging.UnmarshalMessageEvent(record.Value)
	if err != nil {
		logx.WithContext(ctx).Errorf("push: drop malformed toOfflinePush record offset=%d: %v", record.Offset, err)
		return nil
	}
	userIDs := uniqueNonEmpty(event.Payload.ReceiverIDs)
	if len(userIDs) == 0 {
		return nil
	}

	if tc := traceContext(event); tc.TraceID != "" {
		ctx = observability.ContextWithTrace(ctx, tc)
	}
	ctx, span := observability.StartSpan(ctx, "push.offline.vendor")
	defer span.End()

	if err := h.pusher.Push(ctx, userIDs, event); err != nil {
		observability.RecordSpanError(span, err)
		observability.RecordPushOffline(h.pusher.Channel(), "error", len(userIDs))
		return fmt.Errorf("offline push server_msg_id=%s: %w", event.ServerMsgID, err)
	}
	observability.RecordPushOffline(h.pusher.Channel(), "sent", len(userIDs))
	return nil
}
