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
	AcceptTimeout time.Duration
	//
	AcceptorTimeout time.Duration
	// log level
	LogLevel int
	// log output
	LogOutput io.Writer
	// file dir
	FileDir string
	// header name
	DigestHeader string
	// file buffer size in byte
	FileBufferSize uint
	// delay for checking new files
	CheckingDelay time.Duration
}
