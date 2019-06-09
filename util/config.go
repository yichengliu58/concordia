// manipulate configure file

package util

import (
	"io"
	"net"
	"time"
)

type Config struct {
	// unique id for current node
	ID uint8
	// port for interconnections between nodes
	ProtoPort uint16
	// port for accepting request from client
	ServicePort uint16
	// peers addr
	Peers []net.TCPAddr
	// maximum number of concurrent proposal this node can issue
	MaxPendingProposals uint32
	//
	QuorumNumber uint8
	//
	PrepareTimeout time.Duration
	//
	PrepareRetryTimes uint
	//
	AcceptRetryTimes uint
	//
	AcceptTimeout time.Duration
	//
	AcceptorTimeout time.Duration
	// log level
	LogLevel int
	// log output
	LogOutput io.Writer
}
