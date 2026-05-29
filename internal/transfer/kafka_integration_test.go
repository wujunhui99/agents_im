package transfer

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/wujunhui99/agents_im/pkg/messaging"
)

func TestKafkaEventConsumerConsumesRedpandaEvent(t *testing.T) {
	if os.Getenv("KAFKA_REDPANDA_INTEGRATION") != "1" {
		t.Skip("set KAFKA_REDPANDA_INTEGRATION=1 with local Redpanda to run")
	}
	brokers := messaging.ParseBrokerList(os.Getenv("KAFKA_BROKERS"))
	if len(brokers) == 0 {
		t.Skip("KAFKA_BROKERS is required for Redpanda integration test")
	}
	topic := os.Getenv("KAFKA_MESSAGE_EVENTS_TOPIC")
	if topic == "" {
		topic = messaging.DefaultMessageEventsTopic
	}
	group := fmt.Sprintf("%s-integration-%d", messaging.DefaultConsumerGroup, time.Now().UnixNano())

	consumer, err := NewKafkaEventConsumer(KafkaEventConsumerConfig{
		Brokers: brokers,
		Topic:   topic,
		GroupID: group,
	})
	if err != nil {
		t.Fatalf("new kafka event consumer: %v", err)
	}
	defer consumer.Close()

	producer, err := messaging.NewKafkaProducer(messaging.KafkaProducerConfig{
		Brokers: brokers,
		Topic:   topic,
	})
	if err != nil {
		t.Fatalf("new kafka producer: %v", err)
	}
	defer producer.Close()

	event := kafkaConsumerAcceptedEvent()
	event.EventID = fmt.Sprintf("evt_transfer_integration_%d", time.Now().UnixNano())
	event.ServerMsgID = fmt.Sprintf("msg_transfer_integration_%d", time.Now().UnixNano())
	event.CreatedAt = time.Now().UTC().UnixMilli()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	received := make(chan struct {
		envelope Envelope
		err      error
	}, 1)
	go func() {
		for {
			envelope, err := consumer.Receive(ctx)
			if err != nil {
				received <- struct {
					envelope Envelope
					err      error
				}{err: err}
				return
			}
			if envelope.Event.EventID == event.EventID {
				received <- struct {
					envelope Envelope
					err      error
				}{envelope: envelope}
				return
			}
			_ = consumer.MarkSuccessful(ctx, envelope)
		}
	}()

	time.Sleep(250 * time.Millisecond)
	if err := producer.Publish(ctx, event); err != nil {
		t.Fatalf("publish integration event: %v", err)
	}

	result := <-received
	if result.err != nil {
		t.Fatalf("receive integration event: %v", result.err)
	}
	if result.envelope.Event.ServerMsgID != event.ServerMsgID || result.envelope.Event.TraceID != event.Payload.TraceID {
		t.Fatalf("received event mismatch: %+v", result.envelope.Event)
	}
	if err := consumer.MarkSuccessful(ctx, result.envelope); err != nil {
		t.Fatalf("commit integration event offset: %v", err)
	}
}
