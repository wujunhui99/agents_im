package config

import (
	"testing"

	"github.com/zeromicro/go-zero/core/conf"
)

// Regression for #657: local etc/msggateway.yaml carries neither Tracing nor
// Redis blocks; conf.MustLoad must not fail on those optional fields (#655 made
// them required and broke dev-up + prod deploy).
func TestMustLoadLocalMsggatewayYAML(t *testing.T) {
	var c Config
	conf.MustLoad("../../../../etc/msggateway.yaml", &c, conf.UseEnv())
	if c.Name != "msggateway" {
		t.Fatalf("name=%q", c.Name)
	}
}
