package svc

import (
	"testing"

	"github.com/wujunhui99/agents_im/service/user/api/internal/config"
)

func TestNewServiceContextRequiresUserRPCConfig(t *testing.T) {
	_, err := NewServiceContext(config.Config{})
	if err == nil {
		t.Fatalf("NewServiceContext accepted missing UserRPC config")
	}
}
