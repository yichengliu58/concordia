package main

import (
	"concordia/server"
	"concordia/util"
	"fmt"
	"net"
	"os"
	"time"
)

func main() {
	config := &util.Config{
		ID:                  1,
		ProtoPort:           10000,
		ServicePort:         8000,
		Peers:               make([]net.TCPAddr, 0),
		MaxPendingProposals: 100,
		PrepareTimeout:      time.Second * 5,
		AcceptTimeout:       time.Second * 5,
		AcceptorTimeout:     time.Second * 5,
		LogOutput:           os.Stdout,
		LogLevel:            util.ERROR,
		QuorumNumber:        3,
		FileDir:             ".",
		DigestHeader:        "FileDigest",
		DataHeader:          "DataID",
		FileBufferSize:      1024,
		CheckingDelay:       time.Second * 3,
	}

	addr, _ := net.ResolveTCPAddr("tcp", "169.254.80.31:10000")
	config.Peers = append(config.Peers, *addr)
	addr, _ = net.ResolveTCPAddr("tcp", "169.254.9.231:10000")
	config.Peers = append(config.Peers, *addr)
	addr, _ = net.ResolveTCPAddr("tcp", "169.254.169.101:10000")
	config.Peers = append(config.Peers, *addr)

	if err := server.Start(config); err != nil {
		fmt.Println("failed to start server", err)
	}
}
