package deepseek

import (
	"context"
	"fmt"

	einodeepseek "github.com/cloudwego/eino-ext/components/model/deepseek"
	einomodel "github.com/cloudwego/eino/components/model"
	appconfig "github.com/wujunhui99/agents_im/internal/config"
)

func NewChatModel(ctx context.Context, cfg appconfig.DeepSeekConfig) (einomodel.ToolCallingChatModel, error) {
	cfg = appconfig.ResolveDeepSeekConfig(cfg)
	if err := appconfig.ValidateDeepSeekConfig(cfg); err != nil {
		return nil, err
	}

	cm, err := einodeepseek.NewChatModel(ctx, &einodeepseek.ChatModelConfig{
		APIKey:  cfg.APIKey,
		BaseURL: cfg.BaseURL,
		Model:   cfg.Model,
	})
	if err != nil {
		return nil, fmt.Errorf("create deepseek chat model: %w", err)
	}
	return cm, nil
}
