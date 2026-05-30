package observability

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	MetricMessageSends     = "agents_im_message_sends_total"
	MetricDeliveryAttempts = "agents_im_delivery_attempts_total"
	MetricTransferEvents   = "agents_im_transfer_events_total"
	MetricWebSocketCurrent = "agents_im_websocket_connections"
	MetricWebSocketEvents  = "agents_im_websocket_connection_events_total"
	MetricHTTPRequests     = "agents_im_http_requests_total"

	defaultUnknownLabelValue = "unknown"
)

// registry holds only the agents_im business metrics so /metrics output stays
// equivalent to the previous self-implemented exposition (no default Go/process
// collectors). promhttp serves it in the standard Prometheus text format.
var registry = prometheus.NewRegistry()

var (
	messageSends = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: MetricMessageSends,
		Help: "Message send attempts by status and chat type.",
	}, []string{"status", "chat_type"})

	deliveryAttempts = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: MetricDeliveryAttempts,
		Help: "Gateway delivery attempts by result status.",
	}, []string{"status"})

	transferEvents = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: MetricTransferEvents,
		Help: "Message transfer worker events by processing result.",
	}, []string{"result"})

	websocketConnections = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: MetricWebSocketCurrent,
		Help: "Current WebSocket connection count in this process.",
	})

	websocketEvents = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: MetricWebSocketEvents,
		Help: "WebSocket connection lifecycle events.",
	}, []string{"event"})

	httpRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: MetricHTTPRequests,
		Help: "HTTP requests by method, path, and status code.",
	}, []string{"method", "path", "status"})
)

func init() {
	registry.MustRegister(
		messageSends,
		deliveryAttempts,
		transferEvents,
		websocketConnections,
		websocketEvents,
		httpRequests,
	)
}

func MetricsHandler() http.HandlerFunc {
	return promhttp.HandlerFor(registry, promhttp.HandlerOpts{}).ServeHTTP
}

func RecordMessageSend(status string, chatType string) {
	messageSends.WithLabelValues(labelValue(status), labelValue(chatType)).Inc()
}

func RecordDeliveryAttempt(status string) {
	deliveryAttempts.WithLabelValues(labelValue(status)).Inc()
}

func RecordTransferEvent(result string) {
	transferEvents.WithLabelValues(labelValue(result)).Inc()
}

func SetWebSocketConnections(count int) {
	if count < 0 {
		count = 0
	}
	websocketConnections.Set(float64(count))
}

func RecordWebSocketConnectionEvent(event string) {
	websocketEvents.WithLabelValues(labelValue(event)).Inc()
}

func RecordHTTPRequest(method string, path string, status int) {
	httpRequests.WithLabelValues(strings.ToUpper(labelValue(method)), metricPath(path), strconv.Itoa(status)).Inc()
}

func labelValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return defaultUnknownLabelValue
	}
	value = strings.ToLower(value)
	replacer := strings.NewReplacer(" ", "_", "-", "_", ".", "_", "/", "_")
	return replacer.Replace(value)
}

func metricPath(path string) string {
	path = strings.TrimSpace(path)
	switch path {
	case "", "/":
		return "root"
	case "/healthz", "/readyz", "/metrics", "/ws":
		return strings.TrimPrefix(path, "/")
	}

	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
		return defaultUnknownLabelValue
	}
	return strings.ToLower(parts[0])
}
