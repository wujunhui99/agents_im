package main

import (
	"flag"

	groupsentry "github.com/wujunhui99/agents_im/internal/rpcgen/groups/entry"
)

func main() {
	configFile := flag.String("f", "etc/groups-rpc.yaml", "config file")
	flag.Parse()

	groupsentry.Start(*configFile)
}
