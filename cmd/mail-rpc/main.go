package main

import (
	"flag"

	mailentry "github.com/wujunhui99/agents_im/internal/rpcgen/mail/entry"
)

func main() {
	configFile := flag.String("f", "etc/mail-rpc.yaml", "config file")
	flag.Parse()

	mailentry.Start(*configFile)
}
