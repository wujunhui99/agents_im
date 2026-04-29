package messaging

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/segmentio/kafka-go"
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
	value, err := MarshalMessageEvent(event)
	if err != nil {
		return err
	}
	messageTime := time.Now().UTC()
	if event.CreatedAt > 0 {
		messageTime = time.UnixMilli(event.CreatedAt).UTC()
	}
	return p.writer.WriteMessages(ctx, kafka.Message{
		Key:   []byte(event.PartitionKey()),
		Value: value,
		Time:  messageTime,
		Headers: []kafka.Header{
			{Key: "event_type", Value: []byte(event.EventType)},
			{Key: "server_msg_id", Value: []byte(event.ServerMsgID)},
		},
	})
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
