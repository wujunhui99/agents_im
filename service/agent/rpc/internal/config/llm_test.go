package config

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/zeromicro/go-zero/core/conf"
)

// loadLLMObservability/loadDeepSeek 走 go-zero conf 加载路径（生产 conf.MustLoad 同款），
// 验证 default=/options= struct tag 的声明式行为。
func loadDeepSeek(t *testing.T, yaml string) DeepSeekConfig {
	t.Helper()
	var cfg DeepSeekConfig
	if err := conf.LoadFromYamlBytes([]byte(yaml), &cfg); err != nil {
		t.Fatalf("load deepseek config: %v", err)
	}
	return cfg
}

func TestDeepSeekConfigTagDefaults(t *testing.T) {
	cfg := loadDeepSeek(t, "{}\n")
	if cfg.APIKey != "" {
		t.Fatalf("deepseek api key should remain empty without yaml/env, got %q", cfg.APIKey)
	}
	if cfg.BaseURL != DefaultDeepSeekBaseURL {
		t.Fatalf("deepseek base url = %q, want %q", cfg.BaseURL, DefaultDeepSeekBaseURL)
	}
	if cfg.Model != DefaultDeepSeekModel {
		t.Fatalf("deepseek model = %q, want %q", cfg.Model, DefaultDeepSeekModel)
	}
}

func TestLLMObservabilityTagDefaults(t *testing.T) {
	var cfg LLMObservabilityConfig
	if err := conf.LoadFromYamlBytes([]byte("{}\n"), &cfg); err != nil {
		t.Fatalf("load llm observability config: %v", err)
	}
	if cfg.Enabled || cfg.CaptureOutput {
		t.Fatalf("llm observability should stay disabled by default: %+v", cfg)
	}
	if cfg.Backend != LLMObservabilityBackendNoop {
		t.Fatalf("llm observability backend = %q, want %q", cfg.Backend, LLMObservabilityBackendNoop)
	}
	if cfg.MaxOutputBytes != 2048 {
		t.Fatalf("llm observability max output bytes = %d, want 2048", cfg.MaxOutputBytes)
	}
	if cfg.Langfuse.Host != DefaultLangfuseHost {
		t.Fatalf("langfuse host = %q, want %q", cfg.Langfuse.Host, DefaultLangfuseHost)
	}
}

func TestLLMObservabilityRejectsUnsupportedBackend(t *testing.T) {
	var cfg LLMObservabilityConfig
	err := conf.LoadFromYamlBytes([]byte("Backend: langfuze\n"), &cfg)
	if err == nil {
		t.Fatal("expected options= validation to reject unsupported backend")
	}
}

// TestLLMObservabilityEnvTagsUseBareNames 锁定 #664 决策1：env 只 wire 裸名，不再有
// AGENTS_IM_*/LLM_OBS_* 别名。env 覆盖语义由 go-zero 内置（proc.Env 进程级缓存使跨用例
// t.Setenv 不可靠，故用反射核对 tag 字面量）。
func TestLLMObservabilityEnvTagsUseBareNames(t *testing.T) {
	assertEnvTag(t, reflect.TypeOf(LLMObservabilityConfig{}), map[string]string{
		"Enabled":        "LLM_OBSERVABILITY_ENABLED",
		"Backend":        "LLM_OBSERVABILITY_BACKEND",
		"CaptureOutput":  "LLM_OBSERVABILITY_CAPTURE_OUTPUT",
		"MaxOutputBytes": "LLM_OBSERVABILITY_MAX_OUTPUT_BYTES",
	})
	assertEnvTag(t, reflect.TypeOf(LangfuseObservabilityConfig{}), map[string]string{
		"Host":      "LANGFUSE_HOST",
		"PublicKey": "LANGFUSE_PUBLIC_KEY",
		"SecretKey": "LANGFUSE_SECRET_KEY",
	})
	assertEnvTag(t, reflect.TypeOf(DeepSeekConfig{}), map[string]string{
		"APIKey":  "DEEPSEEK_API_KEY",
		"BaseURL": "DEEPSEEK_BASE_URL",
		"Model":   "DEEPSEEK_MODEL",
	})
}

func assertEnvTag(t *testing.T, typ reflect.Type, wantEnv map[string]string) {
	t.Helper()
	for field, env := range wantEnv {
		f, ok := typ.FieldByName(field)
		if !ok {
			t.Fatalf("%s field %s missing", typ.Name(), field)
		}
		tag := f.Tag.Get("json")
		if !strings.Contains(tag, "env="+env) {
			t.Fatalf("%s.%s json tag %q missing env=%s", typ.Name(), field, tag, env)
		}
		if strings.Contains(tag, "AGENTS_IM_") || strings.Contains(tag, "_OBS_") || strings.Contains(tag, "LANGFUSE_BASE_URL") {
			t.Fatalf("%s.%s json tag %q must not keep legacy alias", typ.Name(), field, tag)
		}
	}
}

func TestValidateDeepSeekConfigRequiresAPIKey(t *testing.T) {
	err := ValidateDeepSeekConfig(DeepSeekConfig{BaseURL: DefaultDeepSeekBaseURL, Model: DefaultDeepSeekModel})
	if !errors.Is(err, ErrDeepSeekAPIKeyMissing) {
		t.Fatalf("validate deepseek config error = %v, want %v", err, ErrDeepSeekAPIKeyMissing)
	}
}

func TestValidateDeepSeekConfigRejectsPlaceholderAPIKey(t *testing.T) {
	err := ValidateDeepSeekConfig(DeepSeekConfig{APIKey: "replace-with-local-deepseek-api-key"})
	if !errors.Is(err, ErrDeepSeekAPIKeyPlaceholder) {
		t.Fatalf("validate deepseek config error = %v, want %v", err, ErrDeepSeekAPIKeyPlaceholder)
	}
}
