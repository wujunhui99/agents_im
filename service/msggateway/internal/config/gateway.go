package config

import (
	"fmt"
	"net/url"
	"os"
	"strings"
)

// Gateway WebSocket / 下行推送 gRPC 配置（#663：从 pkg/config 搬到 msggateway 域属主，
// 仅本服务消费）。#664：标量字段的默认值/env 覆盖改用 go-zero struct tag（default=/env=），
// 在 conf.MustLoad 时声明式生效，不再手写 Resolve。两处例外见 NormalizeGatewayWSConfig。
const (
	defaultGatewayWSPingSeconds      = 30
	defaultGatewayWSHeartbeatSeconds = 75
	defaultGatewayWSCommandRate      = 20
	defaultGatewayWSCommandBurst     = 40
)

type GatewayWSConfig struct {
	// AllowedOrigins 是切片且与浏览器 Origin header（外部输入）做相等匹配，go-zero env
	// tag 只支持标量，无法表达；故保留手写 env 读取 + 归一化（见 NormalizeGatewayWSConfig）。
	AllowedOrigins            []string `json:",optional"`
	AllowQueryToken           bool     `json:",optional,env=GATEWAY_WS_ALLOW_QUERY_TOKEN"`
	PingIntervalSeconds       int64    `json:",default=30,env=GATEWAY_WS_PING_INTERVAL_SECONDS"`
	HeartbeatTimeoutSeconds   int64    `json:",default=75,env=GATEWAY_WS_HEARTBEAT_TIMEOUT_SECONDS"`
	CommandRateLimitPerSecond int      `json:",default=20,env=GATEWAY_WS_COMMAND_RATE_LIMIT_PER_SECOND"`
	CommandRateLimitBurst     int      `json:",default=40,env=GATEWAY_WS_COMMAND_RATE_LIMIT_BURST"`
}

type GatewayGRPCConfig struct {
	// ListenOn 形如 0.0.0.0:9100；空则不启用下行推送 gRPC server。
	ListenOn string
}

// DefaultGatewayWSConfig 仅供未走 conf.MustLoad 的运行时兜底使用（server.pingInterval/
// heartbeatTimeout 在字段为零时回落到这里），生产路径的默认值由 struct tag 在 load 时填充。
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

// NormalizeGatewayWSConfig 处理两处 tag 无法表达、必须保留的逻辑：
//  1. AllowedOrigins 归一化（保留例外）：与浏览器发来的 Origin header（外部输入）做相等匹配，
//     config 侧必须与运行时 normalizeRequestOrigin 产出同一规范形，否则合法来源静默匹配失败。
//     env GATEWAY_WS_ALLOWED_ORIGINS（逗号分隔）在 yaml 未配置时兜底。
//  2. ping/heartbeat 跨字段约束改 fail-fast（保留例外）：配错（ping ≥ heartbeat）会让每条 WS
//     连接周期性闪断且无信号，故显式报错而非静默改写。标量默认值/env 覆盖已由 struct tag 在
//     conf.MustLoad 时生效，这里不再重复解析。
func NormalizeGatewayWSConfig(cfg GatewayWSConfig) (GatewayWSConfig, error) {
	origins := cfg.AllowedOrigins
	if len(origins) == 0 {
		origins = originListFromValue(os.Getenv("GATEWAY_WS_ALLOWED_ORIGINS"))
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

	if cfg.PingIntervalSeconds > 0 && cfg.HeartbeatTimeoutSeconds > 0 &&
		cfg.PingIntervalSeconds >= cfg.HeartbeatTimeoutSeconds {
		return cfg, fmt.Errorf("gateway websocket ping interval (%ds) must be smaller than heartbeat timeout (%ds)", cfg.PingIntervalSeconds, cfg.HeartbeatTimeoutSeconds)
	}
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
