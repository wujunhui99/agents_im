package observability

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

const (
	TracingProtocolGRPC = "grpc"
	TracingProtocolHTTP = "http/protobuf"

	DefaultTraceUIBaseURL = "https://grafana.agenticim.xyz"
)

type TracingConfig struct {
	Enabled        bool
	ServiceName    string
	Environment    string
	OTLPEndpoint   string
	Protocol       string
	SamplerRatio   float64
	TraceUIBaseURL string
	Insecure       bool
}

type TracingShutdown func(context.Context) error

var tracingEnabled atomic.Bool

func ResolveTracingConfig(cfg TracingConfig, defaultServiceName string) (TracingConfig, error) {
	if value := firstTracingEnv("AGENTS_IM_TRACING_ENABLED", "TRACING_ENABLED"); value != "" {
		enabled, err := strconv.ParseBool(value)
		if err != nil {
			return cfg, fmt.Errorf("parse tracing enabled: %w", err)
		}
		cfg.Enabled = enabled
	}
	cfg.ServiceName = firstTracingValue(
		strings.TrimSpace(os.ExpandEnv(cfg.ServiceName)),
		firstTracingEnv("AGENTS_IM_TRACING_SERVICE_NAME", "OTEL_SERVICE_NAME"),
		strings.TrimSpace(defaultServiceName),
		"agents-im",
	)
	cfg.Environment = firstTracingValue(
		strings.TrimSpace(os.ExpandEnv(cfg.Environment)),
		firstTracingEnv("AGENTS_IM_ENV", "DEPLOYMENT_ENVIRONMENT"),
		resourceAttributeFromEnv("deployment.environment"),
		"local",
	)
	cfg.OTLPEndpoint = strings.TrimSpace(os.ExpandEnv(firstTracingValue(
		cfg.OTLPEndpoint,
		firstTracingEnv("AGENTS_IM_OTLP_ENDPOINT", "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", "OTEL_EXPORTER_OTLP_ENDPOINT"),
	)))
	cfg.Protocol = normalizeTracingProtocol(firstTracingValue(
		strings.TrimSpace(os.ExpandEnv(cfg.Protocol)),
		firstTracingEnv("AGENTS_IM_OTLP_PROTOCOL", "OTEL_EXPORTER_OTLP_TRACES_PROTOCOL", "OTEL_EXPORTER_OTLP_PROTOCOL"),
		TracingProtocolGRPC,
	))
	if value := firstTracingEnv("AGENTS_IM_TRACING_SAMPLER_RATIO", "OTEL_TRACES_SAMPLER_ARG"); value != "" {
		ratio, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return cfg, fmt.Errorf("parse tracing sampler ratio: %w", err)
		}
		cfg.SamplerRatio = ratio
	}
	if cfg.SamplerRatio == 0 {
		cfg.SamplerRatio = 1
	}
	cfg.TraceUIBaseURL = strings.TrimRight(firstTracingValue(
		strings.TrimSpace(os.ExpandEnv(cfg.TraceUIBaseURL)),
		firstTracingEnv("AGENTS_IM_TRACE_UI_BASE_URL", "GRAFANA_TRACE_UI_BASE_URL"),
		DefaultTraceUIBaseURL,
	), "/")
	if value := firstTracingEnv("AGENTS_IM_OTLP_INSECURE", "OTEL_EXPORTER_OTLP_INSECURE"); value != "" {
		insecure, err := strconv.ParseBool(value)
		if err != nil {
			return cfg, fmt.Errorf("parse tracing insecure: %w", err)
		}
		cfg.Insecure = insecure
	}
	if !strings.HasPrefix(cfg.OTLPEndpoint, "https://") {
		cfg.Insecure = true
	}

	if cfg.Protocol != TracingProtocolGRPC && cfg.Protocol != TracingProtocolHTTP {
		return cfg, fmt.Errorf("unsupported tracing protocol %q", cfg.Protocol)
	}
	if cfg.SamplerRatio < 0 || cfg.SamplerRatio > 1 {
		return cfg, fmt.Errorf("tracing sampler ratio must be between 0 and 1")
	}
	if cfg.Enabled && strings.TrimSpace(cfg.OTLPEndpoint) == "" {
		return cfg, errors.New("tracing OTLP endpoint is required when tracing is enabled")
	}
	return cfg, nil
}

func InitServiceTracing(ctx context.Context, cfg TracingConfig, serviceName string) (TracingShutdown, error) {
	resolved, err := ResolveTracingConfig(cfg, serviceName)
	if err != nil {
		return nil, err
	}
	return initResolvedTracing(ctx, resolved)
}

func initResolvedTracing(ctx context.Context, resolved TracingConfig) (TracingShutdown, error) {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))
	if !resolved.Enabled {
		tracingEnabled.Store(false)
		return func(context.Context) error { return nil }, nil
	}

	exporter, err := newOTLPTraceExporter(ctx, resolved)
	if err != nil {
		return nil, fmt.Errorf("initialize OTLP trace exporter: %w", err)
	}
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(resolved.SamplerRatio))),
		sdktrace.WithResource(resource.NewWithAttributes("",
			attribute.String("service.name", resolved.ServiceName),
			attribute.String("deployment.environment", resolved.Environment),
		)),
	)
	otel.SetTracerProvider(provider)
	tracingEnabled.Store(true)
	return func(shutdownCtx context.Context) error {
		if shutdownCtx == nil {
			var cancel context.CancelFunc
			shutdownCtx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
		}
		tracingEnabled.Store(false)
		return provider.Shutdown(shutdownCtx)
	}, nil
}

func TraceUIBaseURLFromEnv() string {
	return strings.TrimRight(firstTracingValue(firstTracingEnv("AGENTS_IM_TRACE_UI_BASE_URL", "GRAFANA_TRACE_UI_BASE_URL"), DefaultTraceUIBaseURL), "/")
}

func ShutdownTracing(shutdown TracingShutdown) error {
	if shutdown == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return shutdown(ctx)
}

func newOTLPTraceExporter(ctx context.Context, cfg TracingConfig) (*otlptrace.Exporter, error) {
	switch cfg.Protocol {
	case TracingProtocolHTTP:
		opts := []otlptracehttp.Option{}
		if endpointIsURL(cfg.OTLPEndpoint) {
			opts = append(opts, otlptracehttp.WithEndpointURL(cfg.OTLPEndpoint))
		} else {
			opts = append(opts, otlptracehttp.WithEndpoint(cfg.OTLPEndpoint))
		}
		if cfg.Insecure {
			opts = append(opts, otlptracehttp.WithInsecure())
		}
		return otlptracehttp.New(ctx, opts...)
	default:
		opts := []otlptracegrpc.Option{}
		if endpointIsURL(cfg.OTLPEndpoint) {
			opts = append(opts, otlptracegrpc.WithEndpointURL(cfg.OTLPEndpoint))
		} else {
			opts = append(opts, otlptracegrpc.WithEndpoint(cfg.OTLPEndpoint))
		}
		if cfg.Insecure {
			opts = append(opts, otlptracegrpc.WithInsecure())
		}
		return otlptracegrpc.New(ctx, opts...)
	}
}

func normalizeTracingProtocol(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "", "grpc", "otlp/grpc":
		return TracingProtocolGRPC
	case "http", "http/protobuf", "otlp/http", "otlp/http/protobuf":
		return TracingProtocolHTTP
	default:
		return value
	}
}

func endpointIsURL(value string) bool {
	parsed, err := url.Parse(strings.TrimSpace(value))
	return err == nil && parsed.Scheme != "" && parsed.Host != ""
}

func firstTracingEnv(keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return ""
}

func firstTracingValue(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func resourceAttributeFromEnv(name string) string {
	for _, item := range strings.Split(os.Getenv("OTEL_RESOURCE_ATTRIBUTES"), ",") {
		key, value, ok := strings.Cut(strings.TrimSpace(item), "=")
		if ok && strings.TrimSpace(key) == name {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
