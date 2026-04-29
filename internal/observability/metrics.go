package observability

import (
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
)

const (
	MetricMessageSends       = "agents_im_message_sends_total"
	MetricDeliveryAttempts   = "agents_im_delivery_attempts_total"
	MetricTransferEvents     = "agents_im_transfer_events_total"
	MetricWebSocketCurrent   = "agents_im_websocket_connections"
	MetricWebSocketEvents    = "agents_im_websocket_connection_events_total"
	MetricHTTPRequests       = "agents_im_http_requests_total"
	metricTypeCounter        = "counter"
	metricTypeGauge          = "gauge"
	defaultUnknownLabelValue = "unknown"
)

type Labels map[string]string

type metricMeta struct {
	name string
	help string
	typ  string
}

type sampleKey struct {
	name   string
	labels string
}

type sample struct {
	labels Labels
	value  float64
}

type Registry struct {
	mu       sync.RWMutex
	meta     map[string]metricMeta
	counters map[sampleKey]sample
	gauges   map[sampleKey]sample
}

var DefaultRegistry = NewRegistry()

func NewRegistry() *Registry {
	registry := &Registry{
		meta:     make(map[string]metricMeta),
		counters: make(map[sampleKey]sample),
		gauges:   make(map[sampleKey]sample),
	}
	registry.register(MetricMessageSends, "Message send attempts by status and chat type.", metricTypeCounter)
	registry.register(MetricDeliveryAttempts, "Gateway delivery attempts by result status.", metricTypeCounter)
	registry.register(MetricTransferEvents, "Message transfer worker events by processing result.", metricTypeCounter)
	registry.register(MetricWebSocketCurrent, "Current WebSocket connection count in this process.", metricTypeGauge)
	registry.register(MetricWebSocketEvents, "WebSocket connection lifecycle events.", metricTypeCounter)
	registry.register(MetricHTTPRequests, "HTTP requests by method, path, and status code.", metricTypeCounter)
	return registry
}

func (r *Registry) IncCounter(name string, labels Labels, delta float64) {
	if r == nil || delta <= 0 {
		return
	}
	labels = cleanLabels(labels)
	key := sampleKey{name: name, labels: labelsKey(labels)}

	r.mu.Lock()
	defer r.mu.Unlock()
	current := r.counters[key]
	current.labels = labels
	current.value += delta
	r.counters[key] = current
}

func (r *Registry) SetGauge(name string, labels Labels, value float64) {
	if r == nil {
		return
	}
	labels = cleanLabels(labels)
	key := sampleKey{name: name, labels: labelsKey(labels)}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.gauges[key] = sample{
		labels: labels,
		value:  value,
	}
}

func (r *Registry) WritePrometheus(w io.Writer) error {
	if r == nil {
		return nil
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.meta))
	for name := range r.meta {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		meta := r.meta[name]
		if _, err := fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s %s\n", meta.name, meta.help, meta.name, meta.typ); err != nil {
			return err
		}
		samples := r.samplesForLocked(meta)
		for _, sample := range samples {
			if _, err := fmt.Fprintf(w, "%s%s %s\n", meta.name, labelsString(sample.labels), strconv.FormatFloat(sample.value, 'f', -1, 64)); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *Registry) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		if err := r.WritePrometheus(w); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

func MetricsHandler() http.HandlerFunc {
	return DefaultRegistry.Handler()
}

func RecordMessageSend(status string, chatType string) {
	DefaultRegistry.IncCounter(MetricMessageSends, Labels{
		"status":    labelValue(status),
		"chat_type": labelValue(chatType),
	}, 1)
}

func RecordDeliveryAttempt(status string) {
	DefaultRegistry.IncCounter(MetricDeliveryAttempts, Labels{
		"status": labelValue(status),
	}, 1)
}

func RecordTransferEvent(result string) {
	DefaultRegistry.IncCounter(MetricTransferEvents, Labels{
		"result": labelValue(result),
	}, 1)
}

func SetWebSocketConnections(count int) {
	if count < 0 {
		count = 0
	}
	DefaultRegistry.SetGauge(MetricWebSocketCurrent, nil, float64(count))
}

func RecordWebSocketConnectionEvent(event string) {
	DefaultRegistry.IncCounter(MetricWebSocketEvents, Labels{
		"event": labelValue(event),
	}, 1)
}

func RecordHTTPRequest(method string, path string, status int) {
	DefaultRegistry.IncCounter(MetricHTTPRequests, Labels{
		"method": strings.ToUpper(labelValue(method)),
		"path":   metricPath(path),
		"status": strconv.Itoa(status),
	}, 1)
}

func (r *Registry) register(name string, help string, typ string) {
	r.meta[name] = metricMeta{name: name, help: help, typ: typ}
}

func (r *Registry) samplesForLocked(meta metricMeta) []sample {
	source := r.counters
	if meta.typ == metricTypeGauge {
		source = r.gauges
	}

	samples := make([]sample, 0)
	for key, value := range source {
		if key.name == meta.name {
			samples = append(samples, value)
		}
	}
	sort.Slice(samples, func(i, j int) bool {
		return labelsKey(samples[i].labels) < labelsKey(samples[j].labels)
	})
	return samples
}

func cleanLabels(labels Labels) Labels {
	if len(labels) == 0 {
		return nil
	}
	clean := make(Labels, len(labels))
	for key, value := range labels {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		clean[key] = labelValue(value)
	}
	if len(clean) == 0 {
		return nil
	}
	return clean
}

func labelsKey(labels Labels) string {
	if len(labels) == 0 {
		return ""
	}
	keys := make([]string, 0, len(labels))
	for key := range labels {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+labels[key])
	}
	return strings.Join(parts, ",")
}

func labelsString(labels Labels) string {
	if len(labels) == 0 {
		return ""
	}
	keys := make([]string, 0, len(labels))
	for key := range labels {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf(`%s="%s"`, key, escapeLabelValue(labels[key])))
	}
	return "{" + strings.Join(parts, ",") + "}"
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

func escapeLabelValue(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, "\n", `\n`)
	return strings.ReplaceAll(value, `"`, `\"`)
}
