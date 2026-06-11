package chain

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/zeromicro/go-zero/core/logx"
	"golang.org/x/sync/errgroup"

	"github.com/wujunhui99/agents_im/pkg/messaging"
)

// EventProducer is the producer surface the handler needs (real: KafkaProducer).
type EventProducer interface {
	PublishEvent(ctx context.Context, topic string, event messaging.MessageEvent) error
}

// SeqMalloc is the seq allocation surface (real: SeqAllocator).
type SeqMalloc interface {
	Malloc(ctx context.Context, conversationID string, n int64) (int64, error)
}

// ChainStore is the Redis surface (real: Store).
type ChainStore interface {
	DedupGet(ctx context.Context, senderID, clientMsgID string) (*DedupRecord, error)
	DedupSet(ctx context.Context, senderID, clientMsgID string, record DedupRecord) error
	CacheMessages(ctx context.Context, conversationID string, events []messaging.MessageEvent) error
	SetHasRead(ctx context.Context, conversationID, userID string, seq int64) error
}

// TransferHandler is the msg.toTransfer.v1 hot path (03 §5): per polled batch,
// group by conversation, then per conversation — dedup → seq Malloc → Redis
// cache/has-read → produce toPostgres + toPush + agent.trigger. The caller
// commits offsets only after the whole batch returns nil (at-least-once;
// replays converge through the dedup record).
type TransferHandler struct {
	seq      SeqMalloc
	store    ChainStore
	producer EventProducer
	workers  int
}

func NewTransferHandler(seq SeqMalloc, store ChainStore, producer EventProducer, workers int) (*TransferHandler, error) {
	if seq == nil || store == nil || producer == nil {
		return nil, fmt.Errorf("transfer handler requires seq allocator, store and producer")
	}
	if workers <= 0 {
		workers = 8
	}
	return &TransferHandler{seq: seq, store: store, producer: producer, workers: workers}, nil
}

// HandleBatch processes one polled batch. A malformed record is logged and
// skipped (it would otherwise wedge its partition forever); any infrastructure
// error fails the whole batch so it is redelivered.
func (h *TransferHandler) HandleBatch(ctx context.Context, records []*kgo.Record) error {
	byConversation := make(map[string][]messaging.MessageEvent)
	order := make([]string, 0)
	for _, record := range records {
		event, err := messaging.UnmarshalMessageEvent(record.Value)
		if err != nil {
			logx.WithContext(ctx).Errorf("msgtransfer: drop malformed toTransfer record topic=%s partition=%d offset=%d: %v",
				record.Topic, record.Partition, record.Offset, err)
			continue
		}
		if event.EventType != messaging.EventTypeMessageSubmitted {
			logx.WithContext(ctx).Errorf("msgtransfer: drop unexpected event_type %q offset=%d", event.EventType, record.Offset)
			continue
		}
		if _, seen := byConversation[event.ConversationID]; !seen {
			order = append(order, event.ConversationID)
		}
		byConversation[event.ConversationID] = append(byConversation[event.ConversationID], event)
	}
	if len(byConversation) == 0 {
		return nil
	}

	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(h.workers)
	for _, conversationID := range order {
		events := byConversation[conversationID]
		group.Go(func() error {
			return h.handleConversation(groupCtx, conversationID, events)
		})
	}
	return group.Wait()
}

// handleConversation processes one conversation's events in arrival order.
func (h *TransferHandler) handleConversation(ctx context.Context, conversationID string, events []messaging.MessageEvent) error {
	fresh := make([]messaging.MessageEvent, 0, len(events))
	seenClientIDs := make(map[string]struct{}, len(events))
	for _, event := range events {
		// In-batch duplicates (client retry landing twice in one poll).
		clientKey := event.SenderID + ":" + event.Payload.ClientMsgID
		if _, dup := seenClientIDs[clientKey]; dup {
			continue
		}
		seenClientIDs[clientKey] = struct{}{}

		record, err := h.store.DedupGet(ctx, event.SenderID, event.Payload.ClientMsgID)
		if err != nil {
			return err
		}
		if record != nil {
			// Already seq-assigned in an earlier batch (replay or client retry):
			// downstream topics already carry it, nothing to do.
			continue
		}
		fresh = append(fresh, event)
	}
	if len(fresh) == 0 {
		return nil
	}

	firstSeq, err := h.seq.Malloc(ctx, conversationID, int64(len(fresh)))
	if err != nil {
		return err
	}
	accepted := make([]messaging.MessageEvent, 0, len(fresh))
	for i, event := range fresh {
		event.EventType = messaging.EventTypeMessageAccepted
		event.Seq = firstSeq + int64(i)
		event.Payload.ReceiverIDs = deriveReceiverIDs(event)
		accepted = append(accepted, event)
	}

	if err := h.store.CacheMessages(ctx, conversationID, accepted); err != nil {
		return err
	}
	for _, event := range accepted {
		if err := h.store.SetHasRead(ctx, conversationID, event.SenderID, event.Seq); err != nil {
			return err
		}
	}
	for _, event := range accepted {
		for _, topic := range []string{messaging.TopicToPostgres, messaging.TopicToPush, messaging.TopicAgentTrigger} {
			if err := h.producer.PublishEvent(ctx, topic, event); err != nil {
				return fmt.Errorf("produce %s: %w", topic, err)
			}
		}
	}
	// Dedup record last: everything before it is idempotent, so a crash in
	// between replays the message instead of losing it.
	for _, event := range accepted {
		if err := h.store.DedupSet(ctx, event.SenderID, event.Payload.ClientMsgID, DedupRecord{
			ConversationID: conversationID,
			Seq:            event.Seq,
			ServerMsgID:    event.ServerMsgID,
			PayloadHash:    event.Payload.PayloadHash,
		}); err != nil {
			return err
		}
	}
	return nil
}

// deriveReceiverIDs computes push fanout recipients. Unlike the legacy outbox
// path it ALWAYS includes the sender: with the Kafka path the SendMessage ACK
// carries no seq, so the sender's own message_received push is how the client
// reconciles its client_msg_id placeholder (03 §4.2). Clients dedup by
// server_msg_id/client_msg_id.
func deriveReceiverIDs(event messaging.MessageEvent) []string {
	ids := make([]string, 0, len(event.Payload.VisibleUserIDs)+2)
	ids = append(ids, event.SenderID)
	if event.ChatType == messaging.ChatTypeSingle && event.Payload.ReceiverID != "" {
		ids = append(ids, event.Payload.ReceiverID)
	}
	ids = append(ids, event.Payload.VisibleUserIDs...)
	return uniqueSortedNonEmpty(ids)
}

func uniqueSortedNonEmpty(values []string) []string {
	cleaned := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			cleaned = append(cleaned, trimmed)
		}
	}
	sort.Strings(cleaned)
	unique := cleaned[:0]
	previous := ""
	for _, value := range cleaned {
		if value == previous {
			continue
		}
		unique = append(unique, value)
		previous = value
	}
	return append([]string(nil), unique...)
}
