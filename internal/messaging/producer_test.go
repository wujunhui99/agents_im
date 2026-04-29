package messaging

import (
	"context"
	"errors"
	"testing"
)

func TestInMemoryProducerStoresClonedEvents(t *testing.T) {
	producer := NewInMemoryProducer()
	event := sampleAcceptedEvent()

	if err := producer.Publish(context.Background(), event); err != nil {
		t.Fatalf("publish event: %v", err)
	}

	event.Payload.ReceiverIDs[0] = "mutated"
	events := producer.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Payload.ReceiverIDs[0] != "user_b" {
		t.Fatalf("stored event was not cloned: %+v", events[0])
	}

	events[0].Payload.ReceiverIDs[0] = "changed-again"
	if producer.Events()[0].Payload.ReceiverIDs[0] != "user_b" {
		t.Fatal("events snapshot should not mutate producer state")
	}
}

func TestNoopProducerValidatesEvents(t *testing.T) {
	producer := NewNoopProducer()
	event := sampleAcceptedEvent()
	event.EventID = ""

	if err := producer.Publish(context.Background(), event); err == nil {
		t.Fatal("expected invalid event to fail")
	}
}

func TestInMemoryProducerRejectsPublishAfterClose(t *testing.T) {
	producer := NewInMemoryProducer()
	if err := producer.Close(); err != nil {
		t.Fatalf("close producer: %v", err)
	}

	err := producer.Publish(context.Background(), sampleAcceptedEvent())
	if !errors.Is(err, ErrProducerClosed) {
		t.Fatalf("expected ErrProducerClosed, got %v", err)
	}
}

func TestParseBrokerList(t *testing.T) {
	brokers := ParseBrokerList(" localhost:19092, redpanda:9092 ,,")
	if len(brokers) != 2 || brokers[0] != "localhost:19092" || brokers[1] != "redpanda:9092" {
		t.Fatalf("unexpected brokers: %#v", brokers)
	}
}
