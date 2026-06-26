package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	appconfig "github.com/wujunhui99/agents_im/pkg/config"
)

// LLM / DeepSeek 运行时配置（#663：从 pkg/config 搬到 agent 域属主）。通用 env 解析
// 复用 pkg/config 导出的 FirstNonEmpty/ResolveBool/ResolveInt。
const (
	LLMObservabilityBackendNoop     = "noop"
	LLMObservabilityBackendMemory   = "memory"
	LLMObservabilityBackendTest     = "test"
	LLMObservabilityBackendLangfuse = "langfuse"

	defaultLLMObservabilityMaxOutput = 2048

	DefaultDeepSeekBaseURL = "https://api.deepseek.com"
	DefaultDeepSeekModel   = "deepseek-v4-pro"
	DefaultLangfuseHost    = "https://langfuse.agenticim.xyz"
)

var ErrDeepSeekAPIKeyMissing = errors.New("deepseek API key is required: set DEEPSEEK_API_KEY")
var ErrDeepSeekAPIKeyPlaceholder = errors.New("deepseek API key is a placeholder: set a real DEEPSEEK_API_KEY")

type DeepSeekConfig struct {
	APIKey  string
	BaseURL string
	Model   string
}

type LLMObservabilityConfig struct {
	Enabled        bool
	Backend        string
	CaptureOutput  bool
	MaxOutputBytes int
	Langfuse       LangfuseObservabilityConfig
}

type LangfuseObservabilityConfig struct {
	Host      string
	PublicKey string
	SecretKey string
}

func DefaultLLMObservabilityConfig() LLMObservabilityConfig {
	return LLMObservabilityConfig{
		Enabled:        false,
		Backend:        LLMObservabilityBackendNoop,
		MaxOutputBytes: defaultLLMObservabilityMaxOutput,
		Langfuse: LangfuseObservabilityConfig{
			Host: DefaultLangfuseHost,
		},
	}
}

func ResolveDeepSeekConfig(cfg DeepSeekConfig) DeepSeekConfig {
	cfg.APIKey = appconfig.FirstNonEmpty(strings.TrimSpace(os.ExpandEnv(cfg.APIKey)), os.Getenv("DEEPSEEK_API_KEY"))
	cfg.BaseURL = appconfig.FirstNonEmpty(strings.TrimSpace(os.ExpandEnv(cfg.BaseURL)), os.Getenv("DEEPSEEK_BASE_URL"), DefaultDeepSeekBaseURL)
	cfg.Model = appconfig.FirstNonEmpty(strings.TrimSpace(os.ExpandEnv(cfg.Model)), os.Getenv("DEEPSEEK_MODEL"), DefaultDeepSeekModel)
	return cfg
}

func ResolveLLMObservabilityConfig(cfg LLMObservabilityConfig) (LLMObservabilityConfig, error) {
	enabled, err := appconfig.ResolveBool(cfg.Enabled, os.Getenv("LLM_OBSERVABILITY_ENABLED"), os.Getenv("LLM_OBS_ENABLED"), os.Getenv("AGENTS_IM_LLM_OBSERVABILITY_ENABLED"))
	if err != nil {
		return cfg, err
	}
	cfg.Enabled = enabled
	cfg.Backend, err = resolveLLMObservabilityBackend(cfg.Backend)
	if err != nil {
		return cfg, err
	}
	cfg.CaptureOutput, err = appconfig.ResolveBool(cfg.CaptureOutput, os.Getenv("LLM_OBSERVABILITY_CAPTURE_OUTPUT"), os.Getenv("LLM_OBS_CAPTURE_OUTPUT"))
	if err != nil {
		return cfg, err
	}
	maxOutputBytes, err := resolveLLMObservabilityMaxOutputBytes(cfg.MaxOutputBytes)
	if err != nil {
		return cfg, err
	}
	if maxOutputBytes < 0 {
		maxOutputBytes = 0
	}
	if maxOutputBytes == 0 {
		maxOutputBytes = DefaultLLMObservabilityConfig().MaxOutputBytes
	}
	cfg.MaxOutputBytes = maxOutputBytes
	langfuseHost := strings.TrimSpace(os.ExpandEnv(cfg.Langfuse.Host))
	langfuseHostEnv := appconfig.FirstNonEmpty(os.Getenv("LANGFUSE_HOST"), os.Getenv("LANGFUSE_BASE_URL"))
	if langfuseHost == "" || langfuseHost == DefaultLangfuseHost {
		cfg.Langfuse.Host = appconfig.FirstNonEmpty(langfuseHostEnv, langfuseHost, DefaultLangfuseHost)
	} else {
		cfg.Langfuse.Host = langfuseHost
	}
	cfg.Langfuse.PublicKey = appconfig.FirstNonEmpty(strings.TrimSpace(os.ExpandEnv(cfg.Langfuse.PublicKey)), os.Getenv("LANGFUSE_PUBLIC_KEY"))
	cfg.Langfuse.SecretKey = appconfig.FirstNonEmpty(strings.TrimSpace(os.ExpandEnv(cfg.Langfuse.SecretKey)), os.Getenv("LANGFUSE_SECRET_KEY"))
	return cfg, nil
}

func resolveLLMObservabilityBackend(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(os.ExpandEnv(value)))
	envValue := strings.ToLower(strings.TrimSpace(appconfig.FirstNonEmpty(os.Getenv("LLM_OBSERVABILITY_BACKEND"), os.Getenv("LLM_OBS_BACKEND"), os.Getenv("AGENTS_IM_LLM_OBSERVABILITY_BACKEND"))))
	if value == "" || (value == LLMObservabilityBackendNoop && envValue != "") {
		value = envValue
	}
	if value == "" {
		return LLMObservabilityBackendNoop, nil
	}
	switch value {
	case LLMObservabilityBackendNoop:
		return LLMObservabilityBackendNoop, nil
	case LLMObservabilityBackendLangfuse:
		return LLMObservabilityBackendLangfuse, nil
	case LLMObservabilityBackendMemory:
		return LLMObservabilityBackendMemory, nil
	case LLMObservabilityBackendTest:
		return LLMObservabilityBackendTest, nil
	default:
		return "", fmt.Errorf("unsupported llm observability backend %q", value)
	}
}

func resolveLLMObservabilityMaxOutputBytes(current int) (int, error) {
	envValue := appconfig.FirstNonEmpty(os.Getenv("LLM_OBSERVABILITY_MAX_OUTPUT_BYTES"), os.Getenv("LLM_OBS_MAX_OUTPUT_BYTES"))
	if envValue != "" && (current == 0 || current == defaultLLMObservabilityMaxOutput) {
		return strconv.Atoi(strings.TrimSpace(envValue))
	}
	return appconfig.ResolveInt(current, envValue)
}

func ValidateDeepSeekConfig(cfg DeepSeekConfig) error {
	cfg = ResolveDeepSeekConfig(cfg)
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
