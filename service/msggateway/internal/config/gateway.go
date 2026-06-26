package config

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	appconfig "github.com/wujunhui99/agents_im/pkg/config"
)

// Gateway WebSocket / 下行推送 gRPC 配置（#663：从 pkg/config 搬到 msggateway 域属主，
// 仅本服务消费）。通用 env 解析复用 pkg/config 导出的 FirstNonEmpty/ResolveBool/ResolveInt(64)。
const (
	defaultGatewayWSPingSeconds      = 30
	defaultGatewayWSHeartbeatSeconds = 75
	defaultGatewayWSCommandRate      = 20
	defaultGatewayWSCommandBurst     = 40
)

type GatewayWSConfig struct {
	AllowedOrigins            []string
	AllowQueryToken           bool
	PingIntervalSeconds       int64
	HeartbeatTimeoutSeconds   int64
	CommandRateLimitPerSecond int
	CommandRateLimitBurst     int
}

type GatewayGRPCConfig struct {
	// ListenOn 形如 0.0.0.0:9100；空则不启用下行推送 gRPC server。
	ListenOn string
}

func DefaultGatewayWSConfig() GatewayWSConfig {
	return GatewayWSConfig{
		AllowedOrigins:            nil,
		AllowQueryToken:           false,
		PingIntervalSeconds:       defaultGatewayWSPingSeconds,
		HeartbeatTimeoutSeconds:   defaultGatewayWSHeartbeatSeconds,
		CommandRateLimitPerSecond: defaultGatewayWSCommandRate,
		CommandRateLimitBurst:     defaultGatewayWSCommandBurst,
	}
}

func ResolveGatewayWSConfig(cfg GatewayWSConfig) (GatewayWSConfig, error) {
	origins := cfg.AllowedOrigins
	if len(origins) == 0 {
		origins = originListFromValue(appconfig.FirstNonEmpty(os.Getenv("GATEWAY_WS_ALLOWED_ORIGINS"), os.Getenv("AGENTS_IM_GATEWAY_WS_ALLOWED_ORIGINS")))
	}
	normalizedOrigins := make([]string, 0, len(origins))
	seenOrigins := make(map[string]struct{}, len(origins))
	for _, origin := range origins {
		normalized, err := normalizeOrigin(origin)
		if err != nil {
			return cfg, err
		}
		if normalized == "" {
			continue
		}
		if _, ok := seenOrigins[normalized]; ok {
			continue
		}
		seenOrigins[normalized] = struct{}{}
		normalizedOrigins = append(normalizedOrigins, normalized)
	}
	cfg.AllowedOrigins = normalizedOrigins

	allowQueryToken, err := appconfig.ResolveBool(cfg.AllowQueryToken, os.Getenv("GATEWAY_WS_ALLOW_QUERY_TOKEN"), os.Getenv("AGENTS_IM_GATEWAY_WS_ALLOW_QUERY_TOKEN"))
	if err != nil {
		return cfg, err
	}
	cfg.AllowQueryToken = allowQueryToken

	pingInterval, err := appconfig.ResolveInt64(cfg.PingIntervalSeconds, os.Getenv("GATEWAY_WS_PING_INTERVAL_SECONDS"), os.Getenv("AGENTS_IM_GATEWAY_WS_PING_INTERVAL_SECONDS"))
	if err != nil {
		return cfg, err
	}
	if pingInterval <= 0 {
		pingInterval = defaultGatewayWSPingSeconds
	}
	cfg.PingIntervalSeconds = pingInterval

	heartbeatTimeout, err := appconfig.ResolveInt64(cfg.HeartbeatTimeoutSeconds, os.Getenv("GATEWAY_WS_HEARTBEAT_TIMEOUT_SECONDS"), os.Getenv("AGENTS_IM_GATEWAY_WS_HEARTBEAT_TIMEOUT_SECONDS"))
	if err != nil {
		return cfg, err
	}
	if heartbeatTimeout <= 0 {
		heartbeatTimeout = defaultGatewayWSHeartbeatSeconds
	}
	cfg.HeartbeatTimeoutSeconds = heartbeatTimeout
	if cfg.PingIntervalSeconds >= cfg.HeartbeatTimeoutSeconds {
		cfg.PingIntervalSeconds = maxInt64(1, cfg.HeartbeatTimeoutSeconds/2)
	}

	commandRate, err := appconfig.ResolveInt(cfg.CommandRateLimitPerSecond, os.Getenv("GATEWAY_WS_COMMAND_RATE_LIMIT_PER_SECOND"), os.Getenv("AGENTS_IM_GATEWAY_WS_COMMAND_RATE_LIMIT_PER_SECOND"))
	if err != nil {
		return cfg, err
	}
	if commandRate <= 0 {
		commandRate = defaultGatewayWSCommandRate
	}
	cfg.CommandRateLimitPerSecond = commandRate

	commandBurst, err := appconfig.ResolveInt(cfg.CommandRateLimitBurst, os.Getenv("GATEWAY_WS_COMMAND_RATE_LIMIT_BURST"), os.Getenv("AGENTS_IM_GATEWAY_WS_COMMAND_RATE_LIMIT_BURST"))
	if err != nil {
		return cfg, err
	}
	if commandBurst <= 0 {
		commandBurst = maxInt(defaultGatewayWSCommandBurst, commandRate)
	}
	cfg.CommandRateLimitBurst = commandBurst
	return cfg, nil
}

func originListFromValue(value string) []string {
	value = strings.TrimSpace(os.ExpandEnv(value))
	if value == "" {
		return nil
	}
	rawOrigins := strings.Split(value, ",")
	origins := make([]string, 0, len(rawOrigins))
	for _, origin := range rawOrigins {
		origin = strings.TrimSpace(origin)
		if origin != "" {
			origins = append(origins, origin)
		}
	}
	return origins
}

func normalizeOrigin(origin string) (string, error) {
	origin = strings.TrimSpace(os.ExpandEnv(origin))
	if origin == "" {
		return "", nil
	}
	if origin == "*" {
		return "", fmt.Errorf("gateway websocket allowed origin %q is invalid: wildcard origins are not allowed", origin)
	}
	parsed, err := url.Parse(origin)
	if err != nil {
		return "", fmt.Errorf("gateway websocket allowed origin %q is invalid: %w", origin, err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("gateway websocket allowed origin %q is invalid: scheme must be http or https", origin)
	}
	if strings.TrimSpace(parsed.Host) == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") {
		return "", fmt.Errorf("gateway websocket allowed origin %q is invalid: expected exact scheme://host[:port]", origin)
	}
	return strings.ToLower(parsed.Scheme) + "://" + strings.ToLower(parsed.Host), nil
}

func maxInt(a int, b int) int {
	if a > b {
		return a
	}
	return b
}

func maxInt64(a int64, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
