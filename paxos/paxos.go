// multi-paxos implementation without Byzantine fault tolerance

package paxos

import (
	"concordia/util"
	"errors"
	"github.com/valyala/gorpc"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

// enumerations of phase
const (
	PREPARE = iota
	ACCEPT
	COMMIT
)

var log = util.NewLogger("<paxos> ")

// the only message format communicated between paxos nodes
type Message struct {
	// phase of current proposal
	Phase uint8
	// proposal number, starts from each node id (will never be 0)
	Proposal uint32
	// node id for proposer
	Proposer uint8
	// data id
	DataID uint32
	// log index
	LogID uint32
	// paxos value
	Value string
	// paxos arguments
	HighestPrepare uint32
	HighestAccept  uint32
	// whether this request is accepted
	OK bool
	// whether this is a request
	Req bool
}

// used to transfer message from router to paxos goroutines
type chanMsg struct {
	c chan *Message
	m *Message
}

// defines RPC services, also used to do message routing
type rpcRouter struct {
	// routing table for acceptors: dataID|logID -> msg chan
	acceptorTable     map[uint64]chan chanMsg
	acceptorTableLock sync.RWMutex
	// request handler from Node.acceptor()
	requestHandler func(chan chanMsg)
}

// Acceptors use this to receive requests
func (r *rpcRouter) RequestHandler(_ string, req interface{}) interface{} {
	rm := req.(Message)
	m := &rm

	// look up to see if this is for an existing prososal
	tid := uint64(m.DataID<<32) | uint64(m.LogID)
	r.acceptorTableLock.RLock()
	c, ok := r.acceptorTable[tid]
	r.acceptorTableLock.RUnlock()

	if m.Phase == PREPARE || !ok {
		// first proposal, make new acceptor to handle it
		// non-buffered channel
		c = make(chan chanMsg)

		r.acceptorTableLock.Lock()
		r.acceptorTable[tid] = c
		r.acceptorTableLock.Unlock()

		go r.requestHandler(c)
	}

	selfChan := make(chan *Message)
	cm := chanMsg{
		c: selfChan,
		m: m,
	}

	c <- cm
	m = <-selfChan

	return m
}

// all ndoes play as proposer, acceptor and learner at the same time
type Node struct {
	// itself is an rpc server
	self *gorpc.Server
	// each peer is a rpc client on this side
	peers []*gorpc.Client
	// message router
	router *rpcRouter
	// util for this node
	config *util.Config
	// atomic value for concurrent issued proposals
	nPendingProposal uint32
	// start clients once
	startClients sync.Once
}

// Paxos proposer routine
// one gorouine for one log entry of one data item
// there could be multiple proposer runing concurrently, depending on util
func (n *Node) proposer(mchan chan *Message, dataID uint32, logID uint32, v string) (bool, error) {
	// highest accepted value from others, should be the same as v
	var otherValue string
	var highestAccept uint32

	// set initial proposal number
	pid := uint32(n.config.ID)

	pretry := uint(0)
	aretry := uint(0)

	for pretry <= n.config.PrepareRetryTimes && aretry <= n.config.AcceptRetryTimes {
		// update proposal number
		pid += uint32(len(n.peers))

		// make prepare message
		m := &Message{
			Phase:    PREPARE,
			Proposal: pid,
			Proposer: n.config.ID,
			DataID:   dataID,
			LogID:    logID,
			Value:    v,
			Req:      true,
		}

		log.Debugf("proposer %d begins proposal %d for data %d and log %d",
			n.config.ID, pid, m.DataID, m.LogID)

		// prepare phase
		notify1 := n.proposerBroadcast(m)
		successful, retriable := n.proposerWaitPrepare(notify1, &highestAccept, &otherValue)

		if !successful {
			if retriable {
				log.Warnf("proposer %d timedout (%d)s for proposal %d for data %d and log %d with"+
					" value %s in PREPARE phase", n.config.ID, n.config.PrepareTimeout,
					m.Proposal, m.DataID, m.LogID, m.Value)

				continue
			} else {
				log.Warnf("proposer %d failed in proposal %d for data %d and log %d with"+
					" value %s in PREPARE phase",
					n.config.ID, m.Proposal, m.DataID, m.LogID, m.Value)

				return false, errors.New("a newer proposal for this entry was" +
					" proposed in prepare phase")
			}
		}

		// accept phase
		if otherValue != "" && otherValue != v {
			// this means there is another newer proposals sent for the same log entry,
			// this should rarely occur, since we assume one log entry from one
			// data is manipulated by one unique proposer
			// but with re-transmiting requests this might happen
			log.Errorf("[paxos] proposer %d detected its proposal %d with value %s is forced to"+
				" learn another value %s\n", m.Proposer, m.Proposal, m.Value, otherValue)
			m.Value = otherValue
		}

		m.Phase = ACCEPT

		notify2 := n.proposerBroadcast(m)
		successful, retriable = n.proposerWaitAccept(notify2)

		if !successful {
			if retriable {
				log.Warnf("proposer %d timedout (%d)s for proposal %d for data %d and log %d with"+
					" value %s in ACCEPT phase", n.config.ID, n.config.PrepareTimeout,
					m.Proposal, m.DataID, m.LogID, m.Value)
				continue
			} else {
				log.Warnf("proposer %d failed in proposal %d for data %d and log %d with"+
					" value %s in ACCEPT phase",
					n.config.ID, m.Proposal, m.DataID, m.LogID, m.Value)
				return false, errors.New("a newer proposal for this entry was" +
					" proposed in accept phase")
			}
		}

		log.Debugf("proposer %d has collected enough ("+
			"%d in %d) responses for proposal %d with value %s",
			n.config.ID, n.config.QuorumNumber, len(n.peers), m.Proposal, m.Value)

		// broadcast commit
		m.Phase = COMMIT
		n.proposerBroadcast(m)

		return true, nil
	}

	// retry times over limit, failed to propose
	return false, errors.New("retry times over limit")
}

func (n *Node) proposerBroadcast(m *Message) chan *Message {
	notify := make(chan *Message)

	for _, p := range n.peers {
		go func(p *gorpc.Client) {
			res, err := p.Call(m)
			if err != nil {
				notify <- nil
			} else {
				rm := res.(Message)
				notify <- &rm
			}
		}(p)
	}

	return notify
}

func (n *Node) proposerWaitPrepare(notify chan *Message, highestAccept *uint32,
	otherValue *string) (prepared, retriable bool) {
	waiting := true
	count := uint8(0)

	for waiting {
		select {
		case nm := <-notify:
			if nm != nil && nm.OK {
				count++
				if nm.HighestAccept > *highestAccept {
					*highestAccept = nm.HighestAccept
					*otherValue = nm.Value
				}
			}

			if count >= n.config.QuorumNumber-1 {
				// enough response, move to accept phase
				prepared = true
				waiting = false
				break
			}
		case <-time.After(n.config.PrepareTimeout):
			if count >= n.config.QuorumNumber-1 {
				prepared = true
			}
			waiting = false
			retriable = true
			break
		}
	}

	return
}

func (n *Node) proposerWaitAccept(notify chan *Message) (accepted, retriable bool) {
	count := uint8(0)
	waiting := true

	for waiting {
		select {
		case nm := <-notify:
			if nm != nil && nm.OK {
				count++
			}
			if count >= n.config.QuorumNumber-1 {
				waiting = false
				accepted = true
				break
			}
		case <-time.After(n.config.AcceptTimeout):
			if count >= n.config.QuorumNumber-1 {
				accepted = true
			}
			retriable = true
			waiting = false
			break
		}
	}

	return
}

// Paxos acceptor routine
// one acceptor for one log index in one data item
// which may include multiple proposals
func (n *Node) acceptor(mchan chan chanMsg) {
	committed := false

	hp := uint32(0)
	ha := uint32(0)

	var v string

	for !committed {
		select {
		case cm := <-mchan:
			m := cm.m

			if m.Phase == PREPARE {
				n.acceptorPhase1(&hp, &ha, v, m)
				cm.c <- m
			} else if m.Phase == ACCEPT {
				n.acceptorPhase2(&hp, &ha, &v, m)
				cm.c <- m
			} else {
				committed = n.acceptorPhase3(&hp, &ha, &v, m)
				log.Debugf("acceptor %d finialise phase 3 with committed %t for proposal %d with"+
					" value %s", n.config.ID, committed, m.Proposal, m.Value)
			}
		case <-time.After(n.config.AcceptorTimeout):
			log.Warnf("acceptor %d timedout waiting for proposal, exiting", n.config.ID)
			return
		}
	}
}

func (n *Node) acceptorPhase1(hp *uint32, ha *uint32, v string, m *Message) {
	// no record (hp is 0) or this proposal is larger
	if m.Proposal > *hp {
		// accept and promise not to accept smaller proposals
		m.OK = true
		m.HighestPrepare = m.Proposal
		m.HighestAccept = *ha
		m.Value = v

		// set the new proposal to highest
		*hp = m.Proposal
	} else {
		// reject
		m.OK = false
	}

	m.Req = false
}

func (n *Node) acceptorPhase2(hp *uint32, ha *uint32, v *string, m *Message) {
	// if there is no higher prepare during this period
	if m.Proposal >= *hp {
		*ha = m.Proposal
		*hp = m.Proposal
		*v = m.Value

		m.OK = true
	} else {
		// maybe a higher prepare appeared before, reject
		m.OK = false
	}

	m.Req = false
}

func (n *Node) acceptorPhase3(hp *uint32, ha *uint32, v *string, m *Message) bool {
	m.Req = false

	if m.Proposal >= *hp && m.Proposal >= *ha {
		// accept this committed value unconditionally
		*v = m.Value
		m.OK = true
		return true
	}

	m.OK = false
	return false
}

func (n *Node) Propose(dataID uint32, logID uint32, v string) (bool, error) {
	// lazy clients connection
	n.startClients.Do(func() {
		for _, p := range n.peers {
			p.Start()
		}
	})

	if atomic.LoadUint32(&n.nPendingProposal) >= n.config.MaxPendingProposals {
		log.Errorf("[paxos] proposer %d failed to propose, "+
			"concurrent issued proposals over limit\n", n.config.ID)
		return false, errors.New("concurrent issued proposals over limit\n")
	}

	atomic.AddUint32(&n.nPendingProposal, 1)

	// non-buffering channel, force the progress to be synchronous
	mchan := make(chan *Message)

	res, err := n.proposer(mchan, dataID, logID, v)

	return res, err
}

// start paxos node
func (n *Node) Start() error {
	if err := n.self.Start(); err != nil {
		return err
	}

	return nil
}

// create a new paxos node
func NewNode(config *util.Config) *Node {
	gorpc.RegisterType(Message{})
	log.SetOutput(config.LogOutput)
	log.SetLevel(config.LogLevel)

	// setup RPC service
	r := &rpcRouter{
		acceptorTable: make(map[uint64]chan chanMsg),
	}

	// setup RPC server
	s := gorpc.NewTCPServer(":"+strconv.Itoa(int(config.ProtoPort)), r.RequestHandler)

	// setup RPC client
	var c []*gorpc.Client
	for _, v := range config.Peers {
		c = append(c, gorpc.NewTCPClient(v.String()))
	}

	n := &Node{
		self:   s,
		peers:  c,
		router: r,
		config: config,
	}

	// one node object for one machine node, so it's ok to set n.acceptor as handler
	n.router.requestHandler = n.acceptor

	return n
}
