package messaging

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestKafkaProducerPublishesToRedpanda(t *testing.T) {
	if os.Getenv("KAFKA_REDPANDA_INTEGRATION") != "1" {
		t.Skip("set KAFKA_REDPANDA_INTEGRATION=1 with local Redpanda to run")
	}
	brokers := ParseBrokerList(os.Getenv("KAFKA_BROKERS"))
	if len(brokers) == 0 {
		t.Skip("KAFKA_BROKERS is required for Redpanda integration test")
	}
	topic := os.Getenv("KAFKA_MESSAGE_EVENTS_TOPIC")
	if topic == "" {
		topic = DefaultMessageEventsTopic
	}

	producer, err := NewKafkaProducer(KafkaProducerConfig{
		Brokers: brokers,
		Topic:   topic,
	})
	if err != nil {
		t.Fatalf("new kafka producer: %v", err)
	}
	defer producer.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := producer.Publish(ctx, sampleAcceptedEvent()); err != nil {
		t.Fatalf("publish to redpanda: %v", err)
	}
}
