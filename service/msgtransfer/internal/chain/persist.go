package chain

import (
	"context"
	"fmt"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/zeromicro/go-zero/core/logx"

	"github.com/wujunhui99/agents_im/pkg/messaging"
	"github.com/wujunhui99/agents_im/service/msgtransfer/internal/model"
)

// PersistWriter is the PG surface the persist consumer needs (real: model.Writer).
type PersistWriter interface {
	PersistBatch(ctx context.Context, msgs []model.PersistMessage) error
}

// PersistHandler consumes msg.toPostgres.v1 and batch-writes the archive rows
// (03 §3.4: messages 表仅作归档，异步写入). An error fails the batch → Kafka
// redelivers; lag grows visibly instead of dropping data.
type PersistHandler struct {
	writer PersistWriter
}

func NewPersistHandler(writer PersistWriter) (*PersistHandler, error) {
	if writer == nil {
		return nil, fmt.Errorf("persist handler requires a writer")
	}
	return &PersistHandler{writer: writer}, nil
}

func (h *PersistHandler) HandleBatch(ctx context.Context, records []*kgo.Record) error {
	byConversation := make(map[string][]model.PersistMessage)
	order := make([]string, 0)
	for _, record := range records {
		event, err := messaging.UnmarshalMessageEvent(record.Value)
		if err != nil {
			logx.WithContext(ctx).Errorf("msgtransfer: drop malformed toPostgres record offset=%d: %v", record.Offset, err)
			continue
		}
		if event.EventType != messaging.EventTypeMessageAccepted {
			logx.WithContext(ctx).Errorf("msgtransfer: drop unexpected toPostgres event_type %q offset=%d", event.EventType, record.Offset)
			continue
		}
		if _, seen := byConversation[event.ConversationID]; !seen {
			order = append(order, event.ConversationID)
		}
		byConversation[event.ConversationID] = append(byConversation[event.ConversationID], persistMessageFromEvent(event))
	}
	for _, conversationID := range order {
		msgs := byConversation[conversationID]
		if err := h.writer.PersistBatch(ctx, msgs); err != nil {
			if model.IsUniqueViolation(err) {
				logx.WithContext(ctx).Errorf(
					"msgtransfer: SEQ REGRESSION suspected for %s (unique violation persisting assigned seq) — advance redis key msg:seq:conv:%s past max(messages.seq) to recover: %v",
					conversationID, conversationID, err)
			}
			return err
		}
	}
	return nil
}

func persistMessageFromEvent(event messaging.MessageEvent) model.PersistMessage {
	sendTime := time.Time{}
	if event.Payload.SendTime > 0 {
		sendTime = time.UnixMilli(event.Payload.SendTime).UTC()
	} else if event.CreatedAt > 0 {
		sendTime = time.UnixMilli(event.CreatedAt).UTC()
	}
	return model.PersistMessage{
		ServerMsgID:           event.ServerMsgID,
		ClientMsgID:           event.Payload.ClientMsgID,
		SenderID:              event.SenderID,
		ConversationID:        event.ConversationID,
		Seq:                   event.Seq,
		ChatType:              event.ChatType,
		ReceiverID:            event.Payload.ReceiverID,
		GroupID:               event.Payload.GroupID,
		ContentType:           event.Payload.ContentType,
		Content:               string(event.Payload.Content),
		MessageOrigin:         event.Payload.MessageOrigin,
		AgentAccountID:        event.Payload.AgentAccountID,
		TriggerServerMsgID:    event.Payload.TriggerServerMsgID,
		AgentRunID:            event.Payload.AgentRunID,
		AllowRecursiveTrigger: event.Payload.AllowRecursiveTrigger,
		PayloadHash:           event.Payload.PayloadHash,
		SendTime:              sendTime,
		VisibleUserIDs:        append([]string(nil), event.Payload.VisibleUserIDs...),
	}
}
