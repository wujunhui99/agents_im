package transfer

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/segmentio/kafka-go"

	"github.com/wujunhui99/agents_im/internal/messaging"
)

const (
	defaultKafkaConsumerMinBytes = 1
	defaultKafkaConsumerMaxBytes = 10 * 1000 * 1000
	defaultKafkaConsumerMaxWait  = time.Second
)

type KafkaEventConsumerConfig struct {
	Brokers  []string
	Topic    string
	GroupID  string
	MinBytes int
	MaxBytes int
	MaxWait  time.Duration
}

type KafkaEventConsumer struct {
	reader kafkaEventReader
	topic  string
	group  string
}

type kafkaEventReader interface {
	FetchMessage(ctx context.Context) (kafka.Message, error)
	CommitMessages(ctx context.Context, msgs ...kafka.Message) error
	Close() error
}

func NewKafkaEventConsumer(cfg KafkaEventConsumerConfig) (*KafkaEventConsumer, error) {
	brokers := messaging.ParseBrokerList(strings.Join(cfg.Brokers, ","))
	if len(brokers) == 0 {
		return nil, errors.New("kafka brokers are required")
	}
	topic := strings.TrimSpace(cfg.Topic)
	if topic == "" {
		topic = messaging.DefaultMessageEventsTopic
	}
	group := strings.TrimSpace(cfg.GroupID)
	if group == "" {
		group = messaging.DefaultConsumerGroup
	}
	minBytes := cfg.MinBytes
	if minBytes <= 0 {
		minBytes = defaultKafkaConsumerMinBytes
	}
	maxBytes := cfg.MaxBytes
	if maxBytes <= 0 {
		maxBytes = defaultKafkaConsumerMaxBytes
	}
	maxWait := cfg.MaxWait
	if maxWait <= 0 {
		maxWait = defaultKafkaConsumerMaxWait
	}

	return &KafkaEventConsumer{
		reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers:        brokers,
			Topic:          topic,
			GroupID:        group,
			MinBytes:       minBytes,
			MaxBytes:       maxBytes,
			MaxWait:        maxWait,
			CommitInterval: 0,
		}),
		topic: topic,
		group: group,
	}, nil
}

func (c *KafkaEventConsumer) Receive(ctx context.Context) (Envelope, error) {
	if c == nil || c.reader == nil {
		return Envelope{}, errors.New("kafka event consumer is closed")
	}
	message, err := c.reader.FetchMessage(ctx)
	if err != nil {
		return Envelope{}, err
	}
	envelope, err := EnvelopeFromKafkaMessage(message)
	if err != nil {
		return Envelope{}, err
	}
	if envelope.Topic == "" {
		envelope.Topic = c.topic
	}
	if envelope.Key == "" {
		envelope.Key = envelope.Event.ConversationID
	}
	return envelope, nil
}

func (c *KafkaEventConsumer) MarkSuccessful(ctx context.Context, envelope Envelope) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if c == nil || c.reader == nil {
		return errors.New("kafka event consumer is closed")
	}
	return c.reader.CommitMessages(ctx, kafka.Message{
		Topic:     firstTransferString(envelope.Topic, c.topic),
		Partition: int(envelope.Partition),
		Offset:    envelope.Offset,
	})
}

func (c *KafkaEventConsumer) MarkRetry(ctx context.Context, envelope Envelope, decision RetryDecision) error {
	return ctx.Err()
}

func (c *KafkaEventConsumer) MarkFailed(ctx context.Context, envelope Envelope, result ProcessResult) error {
	return ctx.Err()
}

func (c *KafkaEventConsumer) Close() error {
	if c == nil || c.reader == nil {
		return nil
	}
	return c.reader.Close()
}

func EnvelopeFromKafkaMessage(message kafka.Message) (Envelope, error) {
	event, err := messaging.UnmarshalMessageEvent(message.Value)
	if err != nil {
		return Envelope{}, fmt.Errorf("decode kafka message event: %w", err)
	}
	if event.EventType != messaging.EventTypeMessageAccepted {
		return Envelope{}, fmt.Errorf("unsupported transfer event_type %q", event.EventType)
	}

	transferEvent := MessageEvent{
		EventID:        event.EventID,
		EventType:      EventTypeMessageAccepted,
		ConversationID: event.ConversationID,
		Seq:            event.Seq,
		ServerMsgID:    event.ServerMsgID,
		SenderID:       event.SenderID,
		ReceiverIDs:    receiverIDsFromMessagingEvent(event),
		CreatedAt:      event.CreatedAt,
		TraceID:        event.Payload.TraceID,
	}

	return Envelope{
		ID:         event.EventID,
		Topic:      message.Topic,
		Key:        firstTransferString(string(message.Key), event.PartitionKey()),
		Partition:  int32(message.Partition),
		Offset:     message.Offset,
		Attempt:    1,
		ReceivedAt: time.Now().UTC(),
		Event:      transferEvent,
		RawPayload: append([]byte(nil), message.Value...),
	}, nil
}

func receiverIDsFromMessagingEvent(event messaging.MessageEvent) []string {
	receiverIDs := make([]string, 0, len(event.Payload.ReceiverIDs)+1)
	seen := make(map[string]struct{}, len(event.Payload.ReceiverIDs)+1)
	for _, receiverID := range event.Payload.ReceiverIDs {
		receiverID = strings.TrimSpace(receiverID)
		if receiverID == "" {
			continue
		}
		if _, ok := seen[receiverID]; ok {
			continue
		}
		seen[receiverID] = struct{}{}
		receiverIDs = append(receiverIDs, receiverID)
	}
	receiverID := strings.TrimSpace(event.Payload.ReceiverID)
	if receiverID != "" {
		if _, ok := seen[receiverID]; !ok {
			receiverIDs = append(receiverIDs, receiverID)
		}
	}
	return receiverIDs
}

func firstTransferString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
