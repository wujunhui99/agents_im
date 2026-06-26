package deepseek

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"
	appconfig "github.com/wujunhui99/agents_im/service/agent/rpc/internal/config"
)

func TestNewChatModelFailsWhenAPIKeyMissing(t *testing.T) {
	t.Setenv("DEEPSEEK_API_KEY", "")
	t.Setenv("DEEPSEEK_BASE_URL", "")
	t.Setenv("DEEPSEEK_MODEL", "")

	_, err := NewChatModel(context.Background(), appconfig.DeepSeekConfig{})
	if !errors.Is(err, appconfig.ErrDeepSeekAPIKeyMissing) {
		t.Fatalf("NewChatModel error = %v, want %v", err, appconfig.ErrDeepSeekAPIKeyMissing)
	}
}

func TestNewChatModelFailsWhenAPIKeyIsPlaceholder(t *testing.T) {
	t.Setenv("DEEPSEEK_API_KEY", "replace-with-local-deepseek-api-key")
	t.Setenv("DEEPSEEK_BASE_URL", "")
	t.Setenv("DEEPSEEK_MODEL", "")

	_, err := NewChatModel(context.Background(), appconfig.DeepSeekConfig{})
	if !errors.Is(err, appconfig.ErrDeepSeekAPIKeyPlaceholder) {
		t.Fatalf("NewChatModel error = %v, want %v", err, appconfig.ErrDeepSeekAPIKeyPlaceholder)
	}
}

func TestNewChatModelConstructsWithExplicitConfig(t *testing.T) {
	t.Setenv("DEEPSEEK_API_KEY", "")
	t.Setenv("DEEPSEEK_BASE_URL", "")
	t.Setenv("DEEPSEEK_MODEL", "")

	cm, err := NewChatModel(context.Background(), appconfig.DeepSeekConfig{
		APIKey:  "unit-test-deepseek-api-key",
		BaseURL: appconfig.DefaultDeepSeekBaseURL,
		Model:   appconfig.DefaultDeepSeekModel,
	})
	if err != nil {
		t.Fatalf("NewChatModel: %v", err)
	}
	if cm == nil {
		t.Fatal("NewChatModel returned nil chat model")
	}
}

func TestLiveDeepSeekGenerate(t *testing.T) {
	if strings.TrimSpace(os.Getenv("RUN_LIVE_DEEPSEEK_TESTS")) != "1" || strings.TrimSpace(os.Getenv("DEEPSEEK_API_KEY")) == "" {
		t.Skip("set RUN_LIVE_DEEPSEEK_TESTS=1 and DEEPSEEK_API_KEY for live DeepSeek test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	cm, err := NewChatModel(ctx, appconfig.DeepSeekConfig{})
	if err != nil {
		t.Fatalf("NewChatModel: %v", err)
	}
	resp, err := cm.Generate(ctx, []*schema.Message{
		schema.SystemMessage("Reply with a short plain text response."),
		schema.UserMessage("Say pong."),
	})
	if err != nil {
		t.Fatalf("DeepSeek Generate: %v", err)
	}
	if resp == nil || strings.TrimSpace(resp.Content) == "" {
		t.Fatalf("DeepSeek response was empty: %#v", resp)
	}
}
