package main

import (
	"concordia/paxos"
	"concordia/util"
	"fmt"
	"net"
	"os"
	"strconv"
	"time"
)

func main() {
	peers := make([]net.TCPAddr, 0)
	a1, _ := net.ResolveTCPAddr("tcp", "172.31.17.98:10000")
	a2, _ := net.ResolveTCPAddr("tcp", "172.31.21.119:10000")

	peers = append(peers, *a1)
	peers = append(peers, *a2)

	c := util.Config{
		ID:                  1,
		ProtoPort:           10000,
		ServicePort:         8000,
		Peers:               peers,
		MaxPendingProposals: 100,
		QuorumNumber:        2,
		PrepareTimeout:      time.Second * 3,
		AcceptTimeout:       time.Second * 3,
		LogOutput:           os.Stdout,
		LogLevel:            util.DEBUG,
	}

	n := paxos.NewNode(&c)

	if err := n.Start(); err != nil {
		fmt.Println("failed to start node", c.ID)
		os.Exit(1)
	}

	<-time.After(time.Second * 10)
	fmt.Println("begins proposing")

	for i := 0; i < 50; i++ {
		go func(i int) {
			ok, err := n.Propose(uint32(c.ID), uint32(i), "node1")
			if !ok {
				fmt.Println("node" + strconv.Itoa(int(c.ID)) +
					"failed to propose log" + strconv.Itoa(i) + err.Error())
			}
		}(i)
	}

	fmt.Println("waiting results")
	<-time.After(time.Second * 10)
}
