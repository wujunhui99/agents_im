package main

import (
	"flag"

	friendsentry "github.com/wujunhui99/agents_im/service/friends/rpc/entry"
)

func main() {
	configFile := flag.String("f", "etc/friends-rpc.yaml", "config file")
	flag.Parse()

	friendsentry.Start(*configFile)
}
