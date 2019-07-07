package paxos

import (
	"concordia/rsm"
	"concordia/util"
	"math/rand"
	"net"
	"strconv"
	"sync"
	"testing"
	"time"
)

// test non-Byzantine fault
func TestNode_Propose(t *testing.T) {
	c, err := util.ParseConfig("default_conf.json")
	if err != nil {
		t.Fatalf("failed to parse conf: %v", err)
	}
	// modify addrs
	for i := 1; i <= 5; i++ {
		localAddr, _ := net.ResolveTCPAddr("tcp", "127.0.0.1:"+strconv.Itoa(10000+i))
		c.Peers[uint8(i)].Addr = *localAddr
	}
	// no need to change keys
	c1 := *c
	c1.ID = 1
	c1.ProtoPort = 10001

	c2 := c1
	c2.ID = 2
	c2.ProtoPort = 10002

	c3 := c1
	c3.ID = 3
	c3.ProtoPort = 10003

	c4 := c1
	c4.ID = 4
	c4.ProtoPort = 10004

	c5 := c1
	c5.ID = 5
	c5.ProtoPort = 10005

	ns := make([]*Node, 5)

	ns[0] = NewNode(&c1)
	go func() {
		err := ns[0].Start()
		if err != nil {
			t.Fatalf("failed to start node: %s", err.Error())
		}
	}()

	ns[1] = NewNode(&c2)
	go func() {
		err := ns[1].Start()
		if err != nil {
			t.Fatalf("failed to start node: %s", err.Error())
		}
	}()

	ns[2] = NewNode(&c3)
	go func() {
		err := ns[2].Start()
		if err != nil {
			t.Fatalf("failed to start node: %s", err.Error())
		}
	}()

	ns[3] = NewNode(&c4)
	go func() {
		err := ns[3].Start()
		if err != nil {
			t.Fatalf("failed to start node: %s", err.Error())
		}
	}()

	ns[4] = NewNode(&c5)
	go func() {
		err := ns[4].Start()
		if err != nil {
			t.Fatalf("failed to start node: %s", err.Error())
		}
	}()

	wg := sync.WaitGroup{}
	count := 0

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			dataID := rand.Intn(3)
			value := rand.Intn(65536)
			delay := rand.Intn(10)
			node := rand.Intn(3)

			<-time.After(time.Duration(delay) * time.Second)
			ok, err := ns[node].Propose(uint32(dataID), strconv.Itoa(value), "")
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

	for i := uint32(0); i < 3; i++ {
		rr, ok := ns[0].rsm.Load(i)
		if !ok {
			continue
		}

		for _, n := range ns {
			v, ok := n.dataids[i]
			if !ok || v != i {
				t.Fatalf("node %d has incorrect dataids for data %d", n.config.ID, i)
			}
		}

		r := rr.(*rsm.RSM)
		leng := r.Committed.Len()

		c := make([]string, r.Committed.Len())
		for _, v := range r.Committed {
			c[v.ID] = v.Value
		}

		nodecount := 0

		for _, n := range ns {
			rr, ok := n.rsm.Load(i)
			if !ok {
				t.Fatalf("rsm nil")
			}

			for _, v := range r.Committed {
				t.Logf("data %d node %d: [%d] %s", i, n.config.ID, v.ID, v.Value)
			}

			r := rr.(*rsm.RSM)
			diff := false
			if leng == r.Committed.Len() {
				for _, v := range r.Committed {
					if c[v.ID] != v.Value {
						diff = true
					}
				}
			}

			if !diff {
				nodecount++
			}
		}

		if nodecount < int(c1.QuorumNumber) {
			t.Fatalf("valid nodes number is not enough")
		}
	}
}

// test Byzantine fault
func TestNode_Propose2(t *testing.T) {
	c, err := util.ParseConfig("default_conf.json")
	if err != nil {
		t.Fatalf("failed to parse conf: %v", err)
	}
	c.ByzantineFaultTolerance = true
	c.LogLevel = util.DEBUG
	// modify addrs
	for i := 1; i <= 5; i++ {
		localAddr, _ := net.ResolveTCPAddr("tcp", "127.0.0.1:"+strconv.Itoa(10000+i))
		c.Peers[uint8(i)].Addr = *localAddr
	}
	// change keys
	c1 := *c
	c1.ID = 1
	c1.ProtoPort = 10001

	c2 := c1
	c2.ID = 2
	c2.ProtoPort = 10002
	c2.SelfKey, _ = util.ParsePrivateKey(c2.KeyDir + "/2/privatekey.pem")

	c3 := c1
	c3.ID = 3
	c3.ProtoPort = 10003
	c3.SelfKey, _ = util.ParsePrivateKey(c3.KeyDir + "/3/privatekey.pem")

	c4 := c1
	c4.ID = 4
	c4.ProtoPort = 10004
	c4.SelfKey, _ = util.ParsePrivateKey(c4.KeyDir + "/4/privatekey.pem")

	c5 := c1
	c5.ID = 5
	c5.ProtoPort = 10005
	c5.SelfKey, _ = util.ParsePrivateKey(c5.KeyDir + "/5/privatekey.pem")

	ns := make([]*Node, 5)
	waitStart := sync.WaitGroup{}
	waitStart.Add(5)

	ns[0] = NewNode(&c1)
	go func() {
		err := ns[0].Start()
		if err != nil {
			t.Fatalf("failed to start node: %s", err.Error())
		}
		waitStart.Done()
	}()

	ns[1] = NewNode(&c2)
	go func() {
		err := ns[1].Start()
		if err != nil {
			t.Fatalf("failed to start node: %s", err.Error())
		}
		waitStart.Done()
	}()

	ns[2] = NewNode(&c3)
	go func() {
		err := ns[2].Start()
		if err != nil {
			t.Fatalf("failed to start node: %s", err.Error())
		}
		waitStart.Done()
	}()

	ns[3] = NewNode(&c4)
	go func() {
		err := ns[3].Start()
		if err != nil {
			t.Fatalf("failed to start node: %s", err.Error())
		}
		waitStart.Done()
	}()

	ns[4] = NewNode(&c5)
	go func() {
		err := ns[4].Start()
		if err != nil {
			t.Fatalf("failed to start node: %s", err.Error())
		}
		waitStart.Done()
	}()

	waitStart.Wait()

	wg := sync.WaitGroup{}
	count := 0
	// acquire client private key, do not change
	clientPK, _ := util.ParsePrivateKey(c.KeyDir + "/client/privatekey.pem")

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			dataID := rand.Intn(3)
			value := rand.Intn(65536)
			delay := rand.Intn(10)
			node := rand.Intn(3)
			// client signature for value
			signature, _ := util.Sign(strconv.Itoa(value), clientPK)

			<-time.After(time.Duration(delay) * time.Second)
			ok, err := ns[node].Propose(uint32(dataID), strconv.Itoa(value), signature)
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

	for i := uint32(0); i < 3; i++ {
		rr, ok := ns[0].rsm.Load(i)
		if !ok {
			continue
		}

		for _, n := range ns {
			v, ok := n.dataids[i]
			if !ok || v != i {
				t.Fatalf("node %d has incorrect dataids for data %d", n.config.ID, i)
			}
		}

		r := rr.(*rsm.RSM)
		leng := r.Committed.Len()

		c := make([]string, r.Committed.Len())
		for _, v := range r.Committed {
			c[v.ID] = v.Value
		}

		nodecount := 0

		for _, n := range ns {
			rr, ok := n.rsm.Load(i)
			if !ok {
				t.Fatalf("rsm nil")
			}

			for _, v := range r.Committed {
				t.Logf("data %d node %d: [%d] %s", i, n.config.ID, v.ID, v.Value)
			}

			r := rr.(*rsm.RSM)
			diff := false
			if leng == r.Committed.Len() {
				for _, v := range r.Committed {
					if c[v.ID] != v.Value {
						diff = true
					}
				}
			}

			if !diff {
				nodecount++
			}
		}

		if nodecount < int(c1.QuorumNumber) {
			t.Fatalf("valid nodes number is not enough")
		}
	}
}

// test Byzantine fault with evil node
func TestNode_Propose3(t *testing.T) {
	c, err := util.ParseConfig("default_conf.json")
	if err != nil {
		t.Fatalf("failed to parse conf: %v", err)
	}
	c.ByzantineFaultTolerance = true
	c.LogLevel = util.DEBUG
	// modify addrs
	for i := 1; i <= 5; i++ {
		localAddr, _ := net.ResolveTCPAddr("tcp", "127.0.0.1:"+strconv.Itoa(10000+i))
		c.Peers[uint8(i)].Addr = *localAddr
	}
	// change keys
	c1 := *c
	c1.ID = 1
	c1.ProtoPort = 10001

	c2 := c1
	c2.ID = 2
	c2.ProtoPort = 10002
	c2.SelfKey, _ = util.ParsePrivateKey(c2.KeyDir + "/2/privatekey.pem")

	c3 := c1
	c3.ID = 3
	c3.ProtoPort = 10003
	c3.SelfKey, _ = util.ParsePrivateKey(c3.KeyDir + "/3/privatekey.pem")

	c4 := c1
	c4.ID = 4
	c4.ProtoPort = 10004
	c4.SelfKey, _ = util.ParsePrivateKey(c4.KeyDir + "/4/privatekey.pem")

	c5 := c1
	c5.ID = 5
	c5.ProtoPort = 10005
	c5.SelfKey, _ = util.ParsePrivateKey(c5.KeyDir + "/5/privatekey.pem")

	ns := make([]*Node, 5)
	waitStart := sync.WaitGroup{}
	waitStart.Add(5)

	ns[0] = NewNode(&c1)
	go func() {
		err := ns[0].Start()
		if err != nil {
			t.Fatalf("failed to start node: %s", err.Error())
		}
		waitStart.Done()
	}()

	ns[1] = NewNode(&c2)
	go func() {
		err := ns[1].Start()
		if err != nil {
			t.Fatalf("failed to start node: %s", err.Error())
		}
		waitStart.Done()
	}()

	ns[2] = NewNode(&c3)
	go func() {
		err := ns[2].Start()
		if err != nil {
			t.Fatalf("failed to start node: %s", err.Error())
		}
		waitStart.Done()
	}()

	ns[3] = NewNode(&c4)
	go func() {
		err := ns[3].Start()
		if err != nil {
			t.Fatalf("failed to start node: %s", err.Error())
		}
		waitStart.Done()
	}()

	ns[4] = NewNode(&c5)
	go func() {
		err := ns[4].Start()
		if err != nil {
			t.Fatalf("failed to start node: %s", err.Error())
		}
		waitStart.Done()
	}()

	waitStart.Wait()
	// acquire client private key, do not change
	clientPK, _ := util.ParsePrivateKey(c.KeyDir + "/client/privatekey.pem")
	signature, _ := util.Sign("this is the evil message", clientPK)
	evilMsg := &Message{
		Phase:           PREPARE,
		Proposal:        13,
		Proposer:        1,
		DataID:          13,
		LogID:           13,
		Value:           "this is the first evil message",
		ClientSignature: signature,
	}
	evilMsg.SelfSignature = signMessage(evilMsg, c1.SelfKey)
	// send first msg to first 3 nodes
	for i := 0; i < 3; i++ {
		ns[0].peers[i].Send(evilMsg)
	}
	// change to second msg to last 2 ndoes
	evilMsg.Value = "this is the second evil message"
	signature, _ = util.Sign("this is the second evil message", clientPK)
	evilMsg.ClientSignature = signature
	evilMsg.SelfSignature = signMessage(evilMsg, c1.SelfKey)
	for i := 3; i < 5; i++ {
		ns[0].peers[i].Send(evilMsg)
	}
	// set phase to accept
	evilMsg.Phase = ACCEPT
	for i := 0; i < 3; i++ {
		ns[0].peers[i].Send(evilMsg)
	}
	evilMsg.Value = "this is the second evil message"
	signature, _ = util.Sign("this is the second evil message", clientPK)
	evilMsg.ClientSignature = signature
	evilMsg.SelfSignature = signMessage(evilMsg, c1.SelfKey)
	for i := 3; i < 5; i++ {
		ns[0].peers[i].Send(evilMsg)
	}

	// there should be no committed log
	for i := 0; i < 5; i++ {
		if r, ok := ns[0].rsm.Load(uint32(13)); !ok {
			t.Errorf("node %d rsm not found", i)
		} else {
			rr := r.(*rsm.RSM)
			if rr.Committed.Len() > 0 {
				t.Errorf("node %d has committed some value", i)
			}
		}
	}
}
