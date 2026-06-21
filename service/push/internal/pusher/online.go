package pusher

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/zeromicro/go-zero/core/logx"

	"github.com/wujunhui99/agents_im/pkg/messaging"
	"github.com/wujunhui99/agents_im/pkg/observability"
	pushgateway "github.com/wujunhui99/agents_im/service/push/internal/gateway"
)

// Broadcaster is the gateway fan-out surface (real: *gateway.Broadcaster).
type Broadcaster interface {
	Broadcast(ctx context.Context, req pushgateway.PushRequest) (pushgateway.Result, error)
}

// EventProducer publishes the second-stage offline event (real: *messaging.KafkaProducer).
type EventProducer interface {
	PublishEvent(ctx context.Context, topic string, event messaging.MessageEvent) error
}

// OnlineHandler consumes msg.toPush.v1 (03 §6.1): broadcast each accepted message
// to every gateway, then produce msg.toOfflinePush.v1 for recipients that online
// delivery missed (二段式). Batch semantics are at-least-once — any gateway/produce
// error returns non-nil so the whole polled batch is retried (no false offline,
// no silent drop). Malformed records are logged and skipped (they can never
// succeed; blocking the batch on them would wedge the partition).
type OnlineHandler struct {
	broadcaster     Broadcaster
	offlineProducer EventProducer
}

func NewOnlineHandler(broadcaster Broadcaster, offlineProducer EventProducer) (*OnlineHandler, error) {
	if broadcaster == nil || offlineProducer == nil {
		return nil, fmt.Errorf("online handler requires broadcaster and offline producer")
	}
	return &OnlineHandler{broadcaster: broadcaster, offlineProducer: offlineProducer}, nil
}

func (h *OnlineHandler) HandleBatch(ctx context.Context, records []*kgo.Record) error {
	for _, record := range records {
		if err := h.handleOne(ctx, record); err != nil {
			return err
		}
	}
	return nil
}

func (h *OnlineHandler) handleOne(ctx context.Context, record *kgo.Record) error {
	event, err := messaging.UnmarshalMessageEvent(record.Value)
	if err != nil {
		logx.WithContext(ctx).Errorf("push: drop malformed toPush record offset=%d: %v", record.Offset, err)
		return nil
	}
	if event.EventType != messaging.EventTypeMessageAccepted {
		logx.WithContext(ctx).Errorf("push: drop unexpected toPush event_type %q offset=%d", event.EventType, record.Offset)
		return nil
	}

	recipients := pushRecipients(event)
	if len(recipients) == 0 {
		return nil
	}

	if tc := traceContext(event); tc.TraceID != "" {
		ctx = observability.ContextWithTrace(ctx, tc)
	}
	ctx, span := observability.StartSpan(ctx, "push.online.broadcast")
	defer span.End()

	deliveryEvent := deliveryEventFromMessaging(event)
	eventJSON, err := json.Marshal(deliveryEvent)
	if err != nil {
		// A non-encodable event is a programming error, not transient — skip it
		// rather than wedge the partition forever.
		logx.WithContext(ctx).Errorf("push: marshal delivery event server_msg_id=%s: %v", event.ServerMsgID, err)
		return nil
	}

	result, err := h.broadcaster.Broadcast(ctx, pushgateway.PushRequest{
		ConversationID: event.ConversationID,
		RecipientIDs:   recipients,
		Event:          deliveryEvent,
		EventJSON:      eventJSON,
	})
	if err != nil {
		observability.RecordSpanError(span, err)
		return fmt.Errorf("broadcast server_msg_id=%s: %w", event.ServerMsgID, err)
	}

	observability.RecordPushOnline("delivered", len(result.Delivered))
	observability.RecordPushOnline("offline", len(result.OfflineUserIDs))

	if len(result.OfflineUserIDs) == 0 {
		return nil
	}
	offlineEvent := offlinePushEvent(event, result.OfflineUserIDs)
	if err := h.offlineProducer.PublishEvent(ctx, messaging.TopicToOfflinePush, offlineEvent); err != nil {
		observability.RecordSpanError(span, err)
		return fmt.Errorf("produce %s server_msg_id=%s: %w", messaging.TopicToOfflinePush, event.ServerMsgID, err)
	}
	return nil
}

// offlinePushEvent builds the second-stage event: the source message with its
// receiver list narrowed to the recipients online delivery missed. Carries the
// same message.accepted shape (and event_id) so the offline consumer reuses the
// existing unmarshal/validation.
func offlinePushEvent(event messaging.MessageEvent, offlineUserIDs []string) messaging.MessageEvent {
	offline := event.Clone()
	offline.Payload.ReceiverIDs = append([]string(nil), offlineUserIDs...)
	offline.Payload.ReceiverID = ""
	return offline
}
