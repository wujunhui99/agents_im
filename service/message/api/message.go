// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package main

import (
	"flag"
	"fmt"

	appconfig "github.com/wujunhui99/agents_im/internal/config"
	"github.com/wujunhui99/agents_im/service/message/api/internal/config"
	"github.com/wujunhui99/agents_im/service/message/api/internal/handler"
	"github.com/wujunhui99/agents_im/service/message/api/internal/svc"

	"github.com/zeromicro/go-zero/rest"
)

var configFile = flag.String("f", "etc/message-api.yaml", "the config file")

func main() {
	flag.Parse()

	c, err := config.Load(*configFile)
	if err != nil {
		panic(err)
	}

	server := rest.MustNewServer(appconfig.ToRestConf(c))
	defer server.Stop()

	ctx, err := svc.NewServiceContext(c)
	if err != nil {
		panic(err)
	}
	handler.RegisterHandlers(server, ctx)

	fmt.Printf("Starting server at %s:%d...\n", c.Host, c.Port)
	server.Start()
}
