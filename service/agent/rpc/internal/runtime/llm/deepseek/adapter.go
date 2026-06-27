package deepseek

import (
	"context"
	"fmt"
	"strings"

	einodeepseek "github.com/cloudwego/eino-ext/components/model/deepseek"
	einomodel "github.com/cloudwego/eino/components/model"
	appconfig "github.com/wujunhui99/agents_im/service/agent/rpc/internal/config"
)

func NewChatModel(ctx context.Context, cfg appconfig.DeepSeekConfig) (einomodel.ToolCallingChatModel, error) {
	// cfg 已在 ServiceContext 经 conf.MustLoad 由 struct tag 填好默认值/env（#664），此处只校验。
	if err := appconfig.ValidateDeepSeekConfig(cfg); err != nil {
		return nil, err
	}

	cm, err := einodeepseek.NewChatModel(ctx, &einodeepseek.ChatModelConfig{
		APIKey:         cfg.APIKey,
		BaseURL:        cfg.BaseURL,
		Model:          cfg.Model,
		ThinkingConfig: thinkingConfig(cfg.Thinking),
	})
	if err != nil {
		return nil, fmt.Errorf("create deepseek chat model: %w", err)
	}
	return cm, nil
}

// thinkingConfig 把配置的思考模式开关映射为上游 thinking.type；默认/未知值都按非思考处理，
// 显式下发 {type:"disabled"} 以覆盖模型自身默认（避免思考模式带来的额外成本与空回复）。
func thinkingConfig(mode string) *einodeepseek.ThinkingConfig {
	if strings.EqualFold(strings.TrimSpace(mode), appconfig.DeepSeekThinkingEnabled) {
		return &einodeepseek.ThinkingConfig{Type: appconfig.DeepSeekThinkingEnabled}
	}
	return &einodeepseek.ThinkingConfig{Type: appconfig.DeepSeekThinkingDisabled}
}
