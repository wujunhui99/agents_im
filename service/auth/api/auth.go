// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package main

import (
	"flag"

	authentry "github.com/wujunhui99/agents_im/service/auth/api/entry"
)

var configFile = flag.String("f", "etc/auth-api.yaml", "the config file")

func main() {
	flag.Parse()

	authentry.Start(*configFile)
}
