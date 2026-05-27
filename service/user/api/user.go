// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package main

import (
	"flag"

	userentry "github.com/wujunhui99/agents_im/service/user/api/entry"
)

func main() {
	configFile := flag.String("f", "etc/user-api.yaml", "config file")
	flag.Parse()

	userentry.Start(*configFile)
}
