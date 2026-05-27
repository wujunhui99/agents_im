// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package main

import (
	"flag"

	groupsentry "github.com/wujunhui99/agents_im/service/groups/api/entry"
)

func main() {
	configFile := flag.String("f", "etc/groups-api.yaml", "config file")
	flag.Parse()

	groupsentry.Start(*configFile)
}
