package main

import (
	"flag"

	messageentry "github.com/wujunhui99/agents_im/service/message/api/entry"
)

func main() {
	configFile := flag.String("f", "etc/message-api.yaml", "config file")
	flag.Parse()

	messageentry.Start(*configFile)
}
