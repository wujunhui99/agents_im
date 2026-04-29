package main

import (
	"flag"

	messageentry "github.com/wujunhui99/agents_im/internal/rpcgen/message/entry"
)

func main() {
	configFile := flag.String("f", "etc/message-rpc.yaml", "config file")
	flag.Parse()

	messageentry.Start(*configFile)
}
