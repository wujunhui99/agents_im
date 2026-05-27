package main

import (
	"flag"

	authentry "github.com/wujunhui99/agents_im/service/auth/rpc/entry"
)

func main() {
	configFile := flag.String("f", "etc/auth-rpc.yaml", "config file")
	flag.Parse()

	authentry.Start(*configFile)
}
