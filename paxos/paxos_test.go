package paxos

import (
	"concordia/util"
	"net"
	"os"
	"testing"
	"time"
)

var (
	defaultConfig = util.Config{
		ID:                  1,
		ProtoPort:           10000,
		ServicePort:         8000,
		Peers:               make([]net.TCPAddr, 0),
		MaxPendingProposals: 100,
		QuorumNumber:        2,
		PrepareTimeout:      time.Second * 3,
		AcceptTimeout:       time.Second * 3,
		LogOutput:           os.Stdout,
		LogLevel:            util.DEBUG,
	}
)

func TestNode_Propose(t *testing.T) {
	c1 := defaultConfig
	c1.ID = 1
	c1.ProtoPort = 8001
	a, _ := net.ResolveTCPAddr("tcp", "127.0.0.1:8002")
	c1.Peers = append(c1.Peers, *a)
	a, _ = net.ResolveTCPAddr("tcp", "127.0.0.1:8003")
	c1.Peers = append(c1.Peers, *a)

	c2 := defaultConfig
	c2.ID = 2
	c2.ProtoPort = 8002
	a, _ = net.ResolveTCPAddr("tcp", "127.0.0.1:8001")
	c2.Peers = append(c2.Peers, *a)
	a, _ = net.ResolveTCPAddr("tcp", "127.0.0.3:8003")
	c2.Peers = append(c2.Peers, *a)

	c3 := defaultConfig
	c3.ID = 3
	c3.ProtoPort = 8003
	a, _ = net.ResolveTCPAddr("tcp", "127.0.0.2:8001")
	c3.Peers = append(c3.Peers, *a)
	a, _ = net.ResolveTCPAddr("tcp", "127.0.0.3:8002")
	c3.Peers = append(c3.Peers, *a)

	ns := make([]*Node, 3)

	ns[0] = NewNode(&c1)
	err := ns[0].Start()
	if err != nil {
		t.Errorf("failed to start node: %s", err.Error())
	}

	ns[1] = NewNode(&c2)
	err = ns[1].Start()
	if err != nil {
		t.Errorf("failed to start node: %s", err.Error())
	}

	ns[2] = NewNode(&c3)
	err = ns[2].Start()

	if err != nil {
		t.Errorf("failed to start node: %s", err.Error())
	}

	ok, err := ns[0].Propose(1, 1, "caonima")
	if !ok {
		t.Fatalf("proposal failed %s", err.Error())
	}

	<-time.After(time.Second * 5)

	//res := make(chan struct {
	//	ok  bool
	//	err error
	//}, 60)
	//
	//for i := 0; i < 60; i++ {
	//	go func(i int) {
	//		ok, err := ns[0].Propose(1, 10, "caonima")
	//		res <- struct {
	//			ok  bool
	//			err error
	//		}{ok: ok, err: err}
	//	}(i)
	//}
	//
	//for i := 0; i < 60; i++ {
	//	select {
	//	case r := <-res:
	//		if !r.ok {
	//			t.Fatalf("propose failed: %s", r.err.Error())
	//		}
	//	}
	//}
}
