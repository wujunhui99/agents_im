package config

import (
	"errors"
	"strings"
)

// LLM / DeepSeek 运行时配置（#663：从 pkg/config 搬到 agent 域属主）。#664：默认值/env 覆盖
// /枚举校验改用 go-zero struct tag（default=/env=/options=），在 conf.MustLoad 时声明式生效，
// 删掉手写 Resolve*/Default*。#664 决策1：env 只 wire 裸名，不再有 AGENTS_IM_*/LLM_OBS_* 别名。
const (
	LLMObservabilityBackendNoop     = "noop"
	LLMObservabilityBackendMemory   = "memory"
	LLMObservabilityBackendTest     = "test"
	LLMObservabilityBackendLangfuse = "langfuse"

	DefaultDeepSeekBaseURL = "https://api.deepseek.com"
	DefaultDeepSeekModel   = "deepseek-v4-pro"
	DefaultLangfuseHost    = "https://langfuse.agenticim.xyz"
)

var ErrDeepSeekAPIKeyMissing = errors.New("deepseek API key is required: set DEEPSEEK_API_KEY")
var ErrDeepSeekAPIKeyPlaceholder = errors.New("deepseek API key is a placeholder: set a real DEEPSEEK_API_KEY")

type DeepSeekConfig struct {
	APIKey  string `json:",optional,env=DEEPSEEK_API_KEY"`
	BaseURL string `json:",default=https://api.deepseek.com,env=DEEPSEEK_BASE_URL"`
	Model   string `json:",default=deepseek-v4-pro,env=DEEPSEEK_MODEL"`
}

type LLMObservabilityConfig struct {
	Enabled        bool   `json:",optional,env=LLM_OBSERVABILITY_ENABLED"`
	Backend        string `json:",default=noop,options=noop|memory|test|langfuse,env=LLM_OBSERVABILITY_BACKEND"`
	CaptureOutput  bool   `json:",optional,env=LLM_OBSERVABILITY_CAPTURE_OUTPUT"`
	MaxOutputBytes int    `json:",default=2048,env=LLM_OBSERVABILITY_MAX_OUTPUT_BYTES"`
	// Langfuse 不标 optional：让 go-zero 在 yaml 缺省整块时仍下钻填子字段默认值（Host）。
	// 子字段各自 optional/default，块缺省不会报错。
	Langfuse LangfuseObservabilityConfig
}

type LangfuseObservabilityConfig struct {
	Host      string `json:",default=https://langfuse.agenticim.xyz,env=LANGFUSE_HOST"`
	PublicKey string `json:",optional,env=LANGFUSE_PUBLIC_KEY"`
	SecretKey string `json:",optional,env=LANGFUSE_SECRET_KEY"`
}

// ValidateDeepSeekConfig 校验已 load 的 DeepSeek 配置（tag 已填默认值/env），仅做对外占位
// key 检测——这不是防自己，是防把占位符当真 key 静默打到上游。
func ValidateDeepSeekConfig(cfg DeepSeekConfig) error {
	apiKey := strings.TrimSpace(cfg.APIKey)
	if apiKey == "" {
		return ErrDeepSeekAPIKeyMissing
	}
	if isPlaceholderDeepSeekAPIKey(apiKey) {
		return ErrDeepSeekAPIKeyPlaceholder
	}
	return nil
}

func isPlaceholderDeepSeekAPIKey(value string) bool {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch normalized {
	case "replace-with-local-deepseek-api-key",
		"replace-with-your-deepseek-api-key",
		"your-deepseek-api-key",
		"your_deepseek_api_key",
		"deepseek-api-key",
		"test-deepseek-api-key":
		return true
	default:
		return strings.Contains(normalized, "placeholder") || strings.HasPrefix(normalized, "replace-with-")
	}
}
