package config

import (
	"os"
	"strconv"
	"strings"

	"github.com/wujunhui99/agents_im/pkg/observability"
)

// PushConfig drives service/push (03-message-pipeline §6): a Kafka worker that
// consumes msg.toPush.v1 (online broadcast to every msggateway over gRPC) and
// msg.toOfflinePush.v1 (vendor offline push). It owns no database — push is a
// pure delivery scheduler. Loaded with the same flat-YAML whitelist loader as
// MessageTransferConfig so tracing/observability/health stay identical to its
// sibling worker.
type PushConfig struct {
	Name          string
	Tracing       observability.TracingConfig
	Kafka         PushKafkaConfig
	Gateway       PushGatewayConfig
	Observability ObservabilityHTTPConfig
}

type PushKafkaConfig struct {
	// Brokers is a comma-separated bootstrap list, e.g. "redpanda.agents-im.svc.cluster.local:9092".
	Brokers string
}

// PushGatewayConfig points at the msggateway downstream-push gRPC surface. Target
// is a single host:port (resolved to ALL instances via k8s headless-Service DNS,
// no etcd) or an explicit comma-separated host:port list. RefreshSeconds re-resolves
// DNS so new gateway pods join the broadcast set.
type PushGatewayConfig struct {
	Target             string
	RefreshSeconds     int
	DialTimeoutSeconds int
	PushTimeoutSeconds int
}

const (
	defaultPushObservabilityPort  = 8086
	defaultPushGatewayRefreshSecs = 30
	defaultPushGatewayDialSecs    = 5
	defaultPushGatewayPushSecs    = 5
)

func DefaultPushConfig() PushConfig {
	return PushConfig{
		Name:    "push",
		Tracing: observability.TracingConfig{},
		Gateway: PushGatewayConfig{
			RefreshSeconds:     defaultPushGatewayRefreshSecs,
			DialTimeoutSeconds: defaultPushGatewayDialSecs,
			PushTimeoutSeconds: defaultPushGatewayPushSecs,
		},
		Observability: ObservabilityHTTPConfig{
			Enabled: true,
			Host:    defaultTransferObservabilityHost,
			Port:    defaultPushObservabilityPort,
		},
	}
}

// LoadPushConfig reads the flat-YAML config. Like LoadMessageTransferConfig this
// is a whitelist loader: every key must be read explicitly here or it is silently
// dropped (#480). Fail-loud validation (brokers/target required) lives in main.
func LoadPushConfig(path string) (PushConfig, error) {
	cfg := DefaultPushConfig()
	values, err := readFlatYAML(path)
	if err != nil {
		return cfg, err
	}

	if value := values["Name"]; value != "" {
		cfg.Name = strings.TrimSpace(os.ExpandEnv(value))
	}
	cfg.Tracing, err = tracingConfigFromValues(values, cfg.Tracing, cfg.Name)
	if err != nil {
		return cfg, err
	}

	cfg.Kafka.Brokers = FirstNonEmpty(strings.TrimSpace(os.ExpandEnv(values["Kafka.Brokers"])), cfg.Kafka.Brokers)

	cfg.Gateway.Target = FirstNonEmpty(
		strings.TrimSpace(os.ExpandEnv(values["Gateway.Target"])),
		strings.TrimSpace(os.ExpandEnv(values["Gateway.Endpoint"])),
		cfg.Gateway.Target,
	)
	if cfg.Gateway.RefreshSeconds, err = intFromValue(values, "Gateway.RefreshSeconds", cfg.Gateway.RefreshSeconds); err != nil {
		return cfg, err
	}
	if cfg.Gateway.DialTimeoutSeconds, err = intFromValue(values, "Gateway.DialTimeoutSeconds", cfg.Gateway.DialTimeoutSeconds); err != nil {
		return cfg, err
	}
	if cfg.Gateway.PushTimeoutSeconds, err = intFromValue(values, "Gateway.PushTimeoutSeconds", cfg.Gateway.PushTimeoutSeconds); err != nil {
		return cfg, err
	}

	cfg.Observability, err = observabilityHTTPConfigFromValues(values, cfg.Observability)
	if err != nil {
		return cfg, err
	}

	return ResolvePushConfig(cfg), nil
}

// ResolvePushConfig applies env fallbacks + defaults, mirroring the transfer worker.
func ResolvePushConfig(cfg PushConfig) PushConfig {
	cfg.Name = FirstNonEmpty(strings.TrimSpace(os.ExpandEnv(cfg.Name)), "push")
	if tracing, err := observability.ResolveTracingConfig(cfg.Tracing, cfg.Name); err == nil {
		cfg.Tracing = tracing
	}
	cfg.Kafka.Brokers = FirstNonEmpty(strings.TrimSpace(os.ExpandEnv(cfg.Kafka.Brokers)), os.Getenv("KAFKA_BROKERS"))
	cfg.Gateway.Target = FirstNonEmpty(strings.TrimSpace(os.ExpandEnv(cfg.Gateway.Target)), os.Getenv("PUSH_GATEWAY_TARGET"))
	if cfg.Gateway.RefreshSeconds <= 0 {
		cfg.Gateway.RefreshSeconds = defaultPushGatewayRefreshSecs
	}
	if cfg.Gateway.DialTimeoutSeconds <= 0 {
		cfg.Gateway.DialTimeoutSeconds = defaultPushGatewayDialSecs
	}
	if cfg.Gateway.PushTimeoutSeconds <= 0 {
		cfg.Gateway.PushTimeoutSeconds = defaultPushGatewayPushSecs
	}
	if resolved, err := ResolveObservabilityHTTPConfig(cfg.Observability); err == nil {
		cfg.Observability = resolved
	}
	return cfg
}

func intFromValue(values map[string]string, key string, current int) (int, error) {
	raw := strings.TrimSpace(os.ExpandEnv(values[key]))
	if raw == "" {
		return current, nil
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil {
		return current, err
	}
	return parsed, nil
}
