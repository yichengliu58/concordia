package paxos

import (
	"concordia/rsm"
	"concordia/util"
	"math/rand"
	"net"
	"os"
	"strconv"
	"sync"
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
		PrepareTimeout:      time.Second * 5,
		AcceptTimeout:       time.Second * 5,
		AcceptorTimeout:     time.Second * 5,
		LogOutput:           os.Stdout,
		LogLevel:            util.ERROR,
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
	a, _ = net.ResolveTCPAddr("tcp", "127.0.0.1:8001")
	c1.Peers = append(c1.Peers, *a)

	c2 := defaultConfig
	c2.ID = 2
	c2.ProtoPort = 8002
	a, _ = net.ResolveTCPAddr("tcp", "127.0.0.1:8001")
	c2.Peers = append(c2.Peers, *a)
	a, _ = net.ResolveTCPAddr("tcp", "127.0.0.1:8003")
	c2.Peers = append(c2.Peers, *a)
	a, _ = net.ResolveTCPAddr("tcp", "127.0.0.1:8002")
	c2.Peers = append(c2.Peers, *a)

	c3 := defaultConfig
	c3.ID = 3
	c3.ProtoPort = 8003
	a, _ = net.ResolveTCPAddr("tcp", "127.0.0.1:8001")
	c3.Peers = append(c3.Peers, *a)
	a, _ = net.ResolveTCPAddr("tcp", "127.0.0.1:8002")
	c3.Peers = append(c3.Peers, *a)
	a, _ = net.ResolveTCPAddr("tcp", "127.0.0.1:8003")
	c3.Peers = append(c3.Peers, *a)

	ns := make([]*Node, 3)

	ns[0] = NewNode(&c1)
	err := ns[0].Start()
	if err != nil {
		t.Fatalf("failed to start node: %s", err.Error())
	}

	ns[1] = NewNode(&c2)
	err = ns[1].Start()
	if err != nil {
		t.Fatalf("failed to start node: %s", err.Error())
	}

	ns[2] = NewNode(&c3)
	err = ns[2].Start()

	if err != nil {
		t.Fatalf("failed to start node: %s", err.Error())
	}

	wg := sync.WaitGroup{}

	count := 0

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			dataID := rand.Intn(3)
			value := rand.Intn(65536)
			delay := rand.Intn(20)
			node := rand.Intn(3)

			<-time.After(time.Duration(delay) * time.Second)
			ok, err := ns[node].Propose(uint32(dataID), strconv.Itoa(value))
			if !ok {
				t.Logf("node %d PROPOSAL %d FAILED: %s", node, value, err.Error())
			} else {
				//t.Logf("node %d PROPOSAL %d data %d SUCCEED", node, value, dataID)
				count++
			}
			wg.Done()
		}(i)
	}

	wg.Wait()
	<-time.After(time.Second * 3)

	for i := uint32(0); i < 3; i++ {
		rr, ok := ns[0].rsm.Load(i)
		if !ok {
			continue
		}

		r := rr.(*rsm.RSM)
		leng := r.Committed.Len()
		t.Logf("%d length : %d", i, leng)

		c := make([]string, r.Committed.Len())
		for _, v := range r.Committed {
			c[v.ID] = v.Value
		}

		for i, v := range c {
			t.Logf("[%d]: %s", i, v)
		}

		for _, n := range ns {
			rr, ok := n.rsm.Load(i)
			if !ok {
				t.Fatalf("rsm nil")
			}
			r := rr.(*rsm.RSM)
			if leng != r.Committed.Len() {
				t.Fatalf("rsm length not the same %d != %d", leng, r.Committed.Len())
			}

			for _, v := range r.Committed {
				if c[v.ID] != v.Value {
					t.Fatalf("rsm has different values")
				}
			}
		}
	}
}
