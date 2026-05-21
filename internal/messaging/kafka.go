package messaging

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/segmentio/kafka-go"

	"github.com/wujunhui99/agents_im/internal/observability"
)

type KafkaProducerConfig struct {
	Brokers      []string
	Topic        string
	BatchTimeout time.Duration
	WriteTimeout time.Duration
}

type KafkaProducer struct {
	writer *kafka.Writer
}

func NewKafkaProducer(cfg KafkaProducerConfig) (*KafkaProducer, error) {
	brokers := cleanBrokers(cfg.Brokers)
	if len(brokers) == 0 {
		return nil, errors.New("kafka brokers are required")
	}
	topic := strings.TrimSpace(cfg.Topic)
	if topic == "" {
		topic = DefaultMessageEventsTopic
	}
	batchTimeout := cfg.BatchTimeout
	if batchTimeout <= 0 {
		batchTimeout = 10 * time.Millisecond
	}
	writeTimeout := cfg.WriteTimeout
	if writeTimeout <= 0 {
		writeTimeout = 10 * time.Second
	}

	return &KafkaProducer{
		writer: &kafka.Writer{
			Addr:         kafka.TCP(brokers...),
			Topic:        topic,
			Balancer:     &kafka.Hash{},
			RequiredAcks: kafka.RequireAll,
			Async:        false,
			BatchTimeout: batchTimeout,
			WriteTimeout: writeTimeout,
		},
	}, nil
}

func (p *KafkaProducer) Publish(ctx context.Context, event MessageEvent) error {
	if p == nil || p.writer == nil {
		return ErrProducerClosed
	}
	if event.Payload.TraceID == "" {
		traceContext := observability.TraceContextFromContext(ctx)
		event.Payload.TraceID = traceContext.TraceID
		event.Payload.RequestID = traceContext.RequestID
		event.Payload.TraceParent = traceContext.TraceParent
		event.Payload.TraceState = traceContext.TraceState
	}
	message, err := KafkaMessageForEvent(event)
	if err != nil {
		return err
	}
	ctx, span := observability.StartSpan(ctx, "kafka.produce")
	err = p.writer.WriteMessages(ctx, message)
	if err != nil {
		observability.RecordSpanError(span, err)
	}
	span.End()
	return err
}

func (p *KafkaProducer) Close() error {
	if p == nil || p.writer == nil {
		return nil
	}
	return p.writer.Close()
}

func ParseBrokerList(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return cleanBrokers(strings.Split(value, ","))
}

func cleanBrokers(brokers []string) []string {
	cleaned := make([]string, 0, len(brokers))
	for _, broker := range brokers {
		broker = strings.TrimSpace(broker)
		if broker != "" {
			cleaned = append(cleaned, broker)
		}
	}
	return cleaned
}

func KafkaMessageForEvent(event MessageEvent) (kafka.Message, error) {
	value, err := MarshalMessageEvent(event)
	if err != nil {
		return kafka.Message{}, err
	}
	messageTime := time.Now().UTC()
	if event.CreatedAt > 0 {
		messageTime = time.UnixMilli(event.CreatedAt).UTC()
	}
	headers := []kafka.Header{
		{Key: "event_type", Value: []byte(event.EventType)},
		{Key: "server_msg_id", Value: []byte(event.ServerMsgID)},
	}
	if event.Payload.TraceID != "" {
		headers = append(headers, kafka.Header{Key: "x-trace-id", Value: []byte(event.Payload.TraceID)})
	}
	if event.Payload.RequestID != "" {
		headers = append(headers, kafka.Header{Key: "x-request-id", Value: []byte(event.Payload.RequestID)})
	}
	if event.Payload.TraceParent != "" {
		headers = append(headers, kafka.Header{Key: "traceparent", Value: []byte(event.Payload.TraceParent)})
	}
	if event.Payload.TraceState != "" {
		headers = append(headers, kafka.Header{Key: "tracestate", Value: []byte(event.Payload.TraceState)})
	}
	return kafka.Message{
		Key:     []byte(event.PartitionKey()),
		Value:   value,
		Time:    messageTime,
		Headers: headers,
	}, nil
}
