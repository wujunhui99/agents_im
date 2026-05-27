// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package main

import (
	"flag"

	agententry "github.com/wujunhui99/agents_im/service/agent/api/entry"
)

func main() {
	configFile := flag.String("f", "etc/agent-api.yaml", "config file")
	flag.Parse()

	agententry.Start(*configFile)
}
