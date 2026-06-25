package config

import (
	"testing"

	"github.com/zeromicro/go-zero/core/conf"
)

// Regression for #657: local etc/agent-api.yaml carries no Tracing block;
// conf.MustLoad must not fail on the optional Tracing field (#655 made it
// required and broke the prod deploy with `"Tracing" is not set`).
func TestMustLoadLocalAgentAPIYAML(t *testing.T) {
	var c Config
	conf.MustLoad("../../../../../etc/agent-api.yaml", &c, conf.UseEnv())
	if c.Name != "agent-api" {
		t.Fatalf("name=%q", c.Name)
	}
}
