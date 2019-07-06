package main

import (
	"concordia/server"
	"concordia/util"
	"flag"
	"fmt"
)

func main() {
	conf := flag.String("c", "default_conf.json", "configure file path (json)")
	flag.Parse()

	config, err := util.ParseConfig(*conf)
	if err != nil {
		fmt.Println("failed to parse conf:", err)
		return
	}

	if err := server.Start(config); err != nil {
		fmt.Println("failed to start server", err)
	}
}
