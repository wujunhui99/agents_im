package main

import (
	"flag"

	userentry "github.com/wujunhui99/agents_im/internal/rpcgen/user/entry"
)

func main() {
	configFile := flag.String("f", "etc/user-rpc.yaml", "config file")
	flag.Parse()

	userentry.Start(*configFile)
}
