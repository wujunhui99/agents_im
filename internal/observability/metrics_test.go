package observability

import (
	"strings"
	"testing"
)

func TestRegistryWritesPrometheusText(t *testing.T) {
	registry := NewRegistry()
	registry.IncCounter(MetricMessageSends, Labels{"status": "accepted", "chat_type": "single"}, 1)
	registry.IncCounter(MetricDeliveryAttempts, Labels{"status": "delivered"}, 2)
	registry.IncCounter(MetricTransferEvents, Labels{"result": "failed"}, 1)
	registry.SetGauge(MetricWebSocketCurrent, nil, 3)

	var out strings.Builder
	if err := registry.WritePrometheus(&out); err != nil {
		t.Fatalf("write prometheus metrics: %v", err)
	}

	body := out.String()
	for _, expected := range []string{
		"# TYPE agents_im_message_sends_total counter",
		`agents_im_message_sends_total{chat_type="single",status="accepted"} 1`,
		`agents_im_delivery_attempts_total{status="delivered"} 2`,
		`agents_im_transfer_events_total{result="failed"} 1`,
		"agents_im_websocket_connections 3",
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("metrics output missing %q:\n%s", expected, body)
		}
	}
}
