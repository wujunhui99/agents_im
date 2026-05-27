// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package main

import (
	"flag"

	friendsentry "github.com/wujunhui99/agents_im/service/friends/api/entry"
)

func main() {
	configFile := flag.String("f", "etc/friends-api.yaml", "config file")
	flag.Parse()

	friendsentry.Start(*configFile)
}
