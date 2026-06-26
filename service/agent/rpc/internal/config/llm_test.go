package config

import (
	"errors"
	"strings"
	"testing"
)

func TestResolveLLMObservabilityDefaultsLangfuseHost(t *testing.T) {
	t.Setenv("LANGFUSE_HOST", "")
	t.Setenv("LANGFUSE_BASE_URL", "")
	t.Setenv("LANGFUSE_PUBLIC_KEY", "")
	t.Setenv("LANGFUSE_SECRET_KEY", "")

	cfg, err := ResolveLLMObservabilityConfig(LLMObservabilityConfig{})
	if err != nil {
		t.Fatalf("resolve llm observability config: %v", err)
	}
	if cfg.Langfuse.Host != DefaultLangfuseHost {
		t.Fatalf("langfuse host = %q, want %q", cfg.Langfuse.Host, DefaultLangfuseHost)
	}
	if cfg.Enabled || cfg.Backend != LLMObservabilityBackendNoop {
		t.Fatalf("default llm observability should stay disabled noop: %+v", cfg)
	}
}

func TestResolveLLMObservabilityLangfuseHostCanBeOverridden(t *testing.T) {
	t.Setenv("LANGFUSE_HOST", "https://langfuse.override.local")
	t.Setenv("LANGFUSE_BASE_URL", "")

	cfg, err := ResolveLLMObservabilityConfig(DefaultLLMObservabilityConfig())
	if err != nil {
		t.Fatalf("resolve llm observability config: %v", err)
	}
	if cfg.Langfuse.Host != "https://langfuse.override.local" {
		t.Fatalf("langfuse host = %q, want env override", cfg.Langfuse.Host)
	}
}

func TestResolveLLMObservabilityCanEnableLangfuseFromEnv(t *testing.T) {
	t.Setenv("LLM_OBSERVABILITY_ENABLED", "true")
	t.Setenv("LLM_OBSERVABILITY_BACKEND", "langfuse")
	t.Setenv("LLM_OBSERVABILITY_CAPTURE_OUTPUT", "true")
	t.Setenv("LLM_OBSERVABILITY_MAX_OUTPUT_BYTES", "4096")
	t.Setenv("LANGFUSE_HOST", "")
	t.Setenv("LANGFUSE_BASE_URL", "")
	t.Setenv("LANGFUSE_PUBLIC_KEY", "pk-lf-unit-test")
	t.Setenv("LANGFUSE_SECRET_KEY", "sk-lf-unit-test")

	cfg, err := ResolveLLMObservabilityConfig(DefaultLLMObservabilityConfig())
	if err != nil {
		t.Fatalf("resolve llm observability config: %v", err)
	}
	if !cfg.Enabled || cfg.Backend != LLMObservabilityBackendLangfuse {
		t.Fatalf("llm observability should enable langfuse from env: %+v", cfg)
	}
	if !cfg.CaptureOutput || cfg.MaxOutputBytes != 4096 {
		t.Fatalf("capture output settings should resolve from env: %+v", cfg)
	}
	if cfg.Langfuse.Host != DefaultLangfuseHost ||
		cfg.Langfuse.PublicKey != "pk-lf-unit-test" ||
		cfg.Langfuse.SecretKey != "sk-lf-unit-test" {
		t.Fatalf("langfuse env config mismatch: %+v", cfg.Langfuse)
	}
}

func TestResolveLLMObservabilityRejectsUnsupportedBackend(t *testing.T) {
	t.Setenv("LLM_OBSERVABILITY_BACKEND", "langfuze")

	_, err := ResolveLLMObservabilityConfig(DefaultLLMObservabilityConfig())
	if err == nil || !strings.Contains(err.Error(), "unsupported llm observability backend") {
		t.Fatalf("expected unsupported backend error, got %v", err)
	}
}

func TestResolveDeepSeekConfigUsesDefaultsWithoutKey(t *testing.T) {
	t.Setenv("DEEPSEEK_API_KEY", "")
	t.Setenv("DEEPSEEK_BASE_URL", "")
	t.Setenv("DEEPSEEK_MODEL", "")

	cfg := ResolveDeepSeekConfig(DeepSeekConfig{})
	if cfg.APIKey != "" {
		t.Fatalf("deepseek api key should remain empty when env is unset")
	}
	if cfg.BaseURL != DefaultDeepSeekBaseURL {
		t.Fatalf("deepseek base url = %q, want %q", cfg.BaseURL, DefaultDeepSeekBaseURL)
	}
	if cfg.Model != DefaultDeepSeekModel {
		t.Fatalf("deepseek model = %q, want %q", cfg.Model, DefaultDeepSeekModel)
	}
}

func TestValidateDeepSeekConfigRequiresAPIKey(t *testing.T) {
	t.Setenv("DEEPSEEK_API_KEY", "")
	t.Setenv("DEEPSEEK_BASE_URL", "")
	t.Setenv("DEEPSEEK_MODEL", "")

	cfg := ResolveDeepSeekConfig(DeepSeekConfig{})
	err := ValidateDeepSeekConfig(cfg)
	if !errors.Is(err, ErrDeepSeekAPIKeyMissing) {
		t.Fatalf("validate deepseek config error = %v, want %v", err, ErrDeepSeekAPIKeyMissing)
	}
}

func TestValidateDeepSeekConfigRejectsPlaceholderAPIKey(t *testing.T) {
	t.Setenv("DEEPSEEK_API_KEY", "replace-with-local-deepseek-api-key")
	t.Setenv("DEEPSEEK_BASE_URL", "")
	t.Setenv("DEEPSEEK_MODEL", "")

	cfg := ResolveDeepSeekConfig(DeepSeekConfig{})
	err := ValidateDeepSeekConfig(cfg)
	if !errors.Is(err, ErrDeepSeekAPIKeyPlaceholder) {
		t.Fatalf("validate deepseek config error = %v, want %v", err, ErrDeepSeekAPIKeyPlaceholder)
	}
}
