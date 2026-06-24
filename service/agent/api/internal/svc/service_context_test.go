package svc

import (
	"errors"
	"testing"

	apiconfig "github.com/wujunhui99/agents_im/service/agent/api/internal/config"
)

// agent-api 是纯 BFF（#606）：缺 agent-rpc 客户端配置须显式失败（不静默回退）。
func TestNewServiceContextFromConfigRequiresAgentRPC(t *testing.T) {
	_, err := NewServiceContextFromConfig(apiconfig.Config{})
	if !errors.Is(err, ErrAgentRPCConfigRequired) {
		t.Fatalf("expected ErrAgentRPCConfigRequired, got %v", err)
	}
}
