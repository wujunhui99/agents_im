package main

import (
	"flag"

	friendsentry "github.com/wujunhui99/agents_im/internal/rpcgen/friends/entry"
)

func main() {
	configFile := flag.String("f", "etc/friends-rpc.yaml", "config file")
	flag.Parse()

	friendsentry.Start(*configFile)
}
