package config_test

import (
	"testing"

	"github.com/zeromicro/go-zero/core/conf"

	gwconfig "github.com/wujunhui99/agents_im/service/apigateway/internal/config"
)

// Regression for #657: the aggregated apigateway yaml carries no Tracing block for the
// agent domain; conf.MustLoad must not fail on the optional Tracing field (#655 made it
// required and broke the prod deploy with `"Tracing" is not set`).
func TestMustLoadLocalGatewayYAML(t *testing.T) {
	var c gwconfig.Config
	conf.MustLoad("../../../../../service/apigateway/etc/apigateway.yaml", &c, conf.UseEnv())
	if c.Name != "apigateway" {
		t.Fatalf("name=%q", c.Name)
	}
	if len(c.Agent.AgentRPC.Endpoints) == 0 {
		t.Fatalf("agent domain rpc client must be wired, got %+v", c.Agent.AgentRPC)
	}
}
