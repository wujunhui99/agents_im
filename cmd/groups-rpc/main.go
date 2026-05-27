package main

import (
	"flag"

	groupsentry "github.com/wujunhui99/agents_im/service/groups/rpc/entry"
)

func main() {
	configFile := flag.String("f", "etc/groups-rpc.yaml", "config file")
	flag.Parse()

	groupsentry.Start(*configFile)
}
