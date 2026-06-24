package observability

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMetricsHandlerExposesPrometheusText(t *testing.T) {
	RecordMessageSend("accepted", "single")
	RecordDeliveryAttempt("delivered")
	RecordDeliveryAttempt("delivered")
	SetWebSocketConnections(3)

	rec := httptest.NewRecorder()
	MetricsHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	body := rec.Body.String()
	for _, expected := range []string{
		"# TYPE agents_im_message_sends_total counter",
		`agents_im_message_sends_total{chat_type="single",status="accepted"} 1`,
		`agents_im_delivery_attempts_total{status="delivered"} 2`,
		"agents_im_websocket_connections 3",
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("metrics output missing %q:\n%s", expected, body)
		}
	}
}
