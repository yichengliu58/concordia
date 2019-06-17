// multi-paxos implementation without Byzantine fault tolerance

package paxos

import (
	"concordia/rsm"
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

var Logger = util.NewLogger("<paxos>")

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
	requestHandler func(cm chan chanMsg, logID, dataID uint32)
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

	if m.Phase == PREPARE && !ok {
		// first proposal, make new acceptor to handle it
		// non-buffered channel
		c = make(chan chanMsg)

		r.acceptorTableLock.Lock()
		r.acceptorTable[tid] = c
		r.acceptorTableLock.Unlock()

		go r.requestHandler(c, m.LogID, m.DataID)
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
	// each peer is a rpc client on this side, should include itself
	peers []*gorpc.Client
	// message router
	router *rpcRouter
	// util for this node
	config *util.Config
	// atomic value for concurrent issued proposals
	nPendingProposal uint32
	// atomic next proposal number
	nextProposal uint32
	// start clients once
	startClients sync.Once
	// rsm log for each data id
	rsm sync.Map
}

// Paxos proposer routine
// one gorouine for one value (belongs to one log entry) of one data item
// there could be multiple proposer runing concurrently, depending on util
func (n *Node) proposer(mchan chan *Message, dataID uint32, v string) (bool, error) {
	// used to detect if value is changed
	highestAccept := uint32(0)
	otherValue := ""
	valueChanged := false
	// set initial proposal number
	pid := atomic.AddUint32(&n.nextProposal, uint32(len(n.config.Peers)))
	// set rsm for this data id
	rr, _ := n.rsm.LoadOrStore(dataID, &rsm.RSM{})
	// get an available log id
	r := rr.(*rsm.RSM)
	logid := r.Search()
	// make prepare message
	m := &Message{
		Phase:    PREPARE,
		Proposal: pid,
		Proposer: n.config.ID,
		DataID:   dataID,
		LogID:    logid,
		Value:    v,
	}

	Logger.Debugf("proposer %d started proposal %d for data %d log %d value %s",
		n.config.ID, pid, m.DataID, m.LogID, v)

	// prepare phase
	notify1 := n.proposerBroadcast(m)
	successful, timedout := n.proposerWaitPrepare(notify1, &highestAccept, &otherValue)
	if !successful {
		if timedout {
			// didn't receive enough response within timeout
			Logger.Warnf("proposer %d timedout ("+
				"%f)s for proposal %d for data %d log %d with value %s in PREPARE phase",
				n.config.ID, n.config.PrepareTimeout.Seconds(),
				m.Proposal, m.DataID, m.LogID, m.Value)
			return false, errors.New("prepare timedout")
		} else {
			// received reject
			Logger.Warnf("proposer %d was rejected in proposal %d for data %d log %d with"+
				" value %s in PREPARE phase",
				n.config.ID, m.Proposal, m.DataID, m.LogID, m.Value)
			return false, errors.New("prepare rejected")
		}
	}

	// accept phase
	if otherValue != "" && otherValue != v {
		// this means there is another newer proposals sent for the same log entry,
		// this should rarely occur, since we assume one log entry from one
		// data is manipulated by one unique proposer
		// but with re-transmiting requests this might happen
		Logger.Errorf("[paxos] proposer %d detected its proposal %d with value %s is forced to"+
			" learn another value %s\n", m.Proposer, m.Proposal, m.Value, otherValue)
		valueChanged = true
		m.Value = otherValue
	}

	m.Phase = ACCEPT

	notify2 := n.proposerBroadcast(m)
	successful, timedout = n.proposerWaitAccept(notify2)
	if !successful {
		if timedout {
			Logger.Warnf("proposer %d timedout (%d)s for proposal %d for data %d log %d with"+
				" value %s in ACCEPT phase", n.config.ID, n.config.PrepareTimeout,
				m.Proposal, m.DataID, m.LogID, m.Value)
			return false, errors.New("accept timedout")
		} else {
			Logger.Warnf("proposer %d failed in proposal %d for data %d log %d with"+
				" value %s in ACCEPT phase, retrying",
				n.config.ID, m.Proposal, m.DataID, m.LogID, m.Value)
			return false, errors.New("accept rejected")
		}
	}

	// broadcast commit
	m.Phase = COMMIT
	n.proposerBroadcast(m)
	Logger.Debugf("proposer %d has collected enough responses and committed proposal %d for"+
		" log id %d value %s", n.config.ID, m.Proposal, m.LogID, m.Value)

	if valueChanged {
		return true, errors.New("value changed")
	} else {
		return true, nil
	}
}

func (n *Node) proposerBroadcast(m *Message) chan *Message {
	notify := make(chan *Message, len(n.peers))

	for _, p := range n.peers {
		Logger.Debugf("proposer %d is sending proposal %d for data %d log %d to %s",
			n.config.ID, m.Proposal, m.DataID, m.LogID, p.Addr)
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
	otherValue *string) (prepared, timedout bool) {

	waiting := true
	count := uint8(0)
	fail := uint8(0)

	for waiting {
		select {
		case nm := <-notify:
			Logger.Debugf("proposer %d received message: proposal %d from %d for data %d log %d"+
				" with ok %t count %d",
				n.config.ID, nm.Proposal, nm.Proposer, nm.DataID, nm.LogID, nm.OK, count)
			if nm == nil || !nm.OK {
				// sending failed or rejected
				fail++
				if fail >= n.config.QuorumNumber {
					waiting = false
					break
				}
			} else {
				// nm != nil && nm.OK
				count++
				if nm.HighestAccept > *highestAccept {
					*highestAccept = nm.HighestAccept
					*otherValue = nm.Value
				}

				if count >= n.config.QuorumNumber {
					// enough response, move to accept phase
					prepared = true
					waiting = false
					break
				}
			}
		case <-time.After(n.config.PrepareTimeout):
			waiting = false
			if count >= n.config.QuorumNumber {
				prepared = true
			} else {
				timedout = true
			}
			break
		}
	}

	return
}

func (n *Node) proposerWaitAccept(notify chan *Message) (accepted, timedout bool) {
	count := uint8(0)
	fail := uint8(0)
	waiting := true

	for waiting {
		select {
		case nm := <-notify:
			if nm == nil || !nm.OK {
				fail++
				if fail >= n.config.QuorumNumber {
					accepted = false
					waiting = false
					break
				}
			} else {
				count++
				if count >= n.config.QuorumNumber {
					waiting = false
					accepted = true
					break
				}
			}
		case <-time.After(n.config.AcceptTimeout):
			waiting = false
			if count >= n.config.QuorumNumber {
				accepted = true
			} else {
				timedout = true
			}
			break
		}
	}

	return
}

// Paxos acceptor routine
// one acceptor for one log index in one data item
// which may include multiple proposals
func (n *Node) acceptor(mchan chan chanMsg, logID, dataID uint32) {
	committed := false

	Logger.Debugf("acceptor %d started one for data %d log %d", n.config.ID, dataID, logID)

	ha := uint32(0)
	hp := uint32(0)
	v := ""
	rr, _ := n.rsm.LoadOrStore(dataID, &rsm.RSM{})
	r := rr.(*rsm.RSM)
	e := r.Insert(logID)

	if e == nil {
		// this entry has been committed before, reject this proposal
		cm := <-mchan
		cm.m.OK = false
		Logger.Warnf("acceptor %d detected a obsolete proposal %d from proposer %d for data %d and"+
			" log %d with value %s",
			n.config.ID, cm.m.Proposal, cm.m.Proposer, cm.m.DataID, cm.m.LogID, cm.m.Value)
		cm.c <- cm.m
		return
	}

	for !committed {
		select {
		case cm := <-mchan:
			m := cm.m
			Logger.Debugf("acceptor %d received message: proposal %d from proposer %d for data %d"+
				" log %d, representing with entry %p with id %d",
				n.config.ID, m.Proposal, m.Proposer, m.DataID, m.LogID, e, e.ID)

			if m.Phase == PREPARE {
				n.acceptorPhase1(&hp, &ha, v, m)
				Logger.Debugf("acceptor %d finished phase 1 for proposal %d from %d for data %d"+
					" log %d with hp %d ha %d value %s, res is %t",
					n.config.ID, m.Proposal, m.Proposer, m.DataID, m.LogID, hp, ha, v, m.OK)
				cm.c <- m
			} else if m.Phase == ACCEPT {
				n.acceptorPhase2(&hp, &ha, &v, m)
				Logger.Debugf("acceptor %d finished phase 2 for proposal %d from %d for data %d"+
					" log %d with hp %d ha %d value %s, res is %t",
					n.config.ID, m.Proposal, m.Proposer, m.DataID, m.LogID, hp, ha, v, m.OK)
				cm.c <- m
			} else {
				committed = n.acceptorPhase3(&hp, &ha, &v, m)
				if committed {
					e.Value = v
					r.Commit(e)

					Logger.Debugf("acceptor %d finshed phase 3 with committed %t for proposal %d data"+
						" %d log %d value %s",
						n.config.ID, committed, m.Proposal, m.DataID, m.LogID, m.Value)

					tid := uint64(dataID<<32) | uint64(logID)
					n.router.acceptorTableLock.Lock()
					delete(n.router.acceptorTable, tid)
					n.router.acceptorTableLock.Unlock()

					return
				}
			}
		case <-time.After(n.config.AcceptorTimeout):
			Logger.Warnf("acceptor %d timedout waiting for proposal, exiting", n.config.ID)
			r.Free(e)

			tid := uint64(dataID<<32) | uint64(logID)
			n.router.acceptorTableLock.Lock()
			delete(n.router.acceptorTable, tid)
			n.router.acceptorTableLock.Unlock()

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
		if v != "" {
			m.Value = v
		}
		// set the new proposal to highest
		*hp = m.Proposal
	} else {
		// reject
		m.OK = false
	}
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
}

func (n *Node) acceptorPhase3(hp *uint32, ha *uint32, v *string, m *Message) bool {
	if m.Proposal >= *hp && m.Proposal >= *ha {
		// accept this committed value unconditionally
		*v = m.Value
		m.OK = true
		return true
	}

	m.OK = false
	return false
}

// given a value, try best to put it in a log entry
func (n *Node) Propose(dataID uint32, v string) (bool, error) {
	// lazy clients connection
	n.startClients.Do(func() {
		for _, p := range n.peers {
			p.Start()
		}
	})

	if atomic.LoadUint32(&n.nPendingProposal) >= n.config.MaxPendingProposals {
		Logger.Errorf("[paxos] proposer %d failed to propose, "+
			"concurrent issued proposals over limit\n", n.config.ID)
		return false, errors.New("concurrent issued proposals over limit\n")
	}

	atomic.AddUint32(&n.nPendingProposal, 1)

	// non-buffering channel, force the progress to be synchronous
	mchan := make(chan *Message)

	res, err := n.proposer(mchan, dataID, v)

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
	Logger.SetOutput(config.LogOutput)
	Logger.SetLevel(config.LogLevel)
	rsm.Logger.SetOutput(config.LogOutput)
	rsm.Logger.SetLevel(config.LogLevel)

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

	atomic.StoreUint32(&n.nextProposal, uint32(n.config.ID))

	// one node object for one machine node, so it's ok to set n.acceptor as handler
	n.router.requestHandler = n.acceptor

	return n
}