// manipulate configure file

package util

import (
	"crypto/rsa"
	"errors"
	"github.com/tidwall/gjson"
	"io"
	"io/ioutil"
	"net"
	"os"
	"time"
)

type PeerConfig struct {
	// peer's ip address
	Addr net.TCPAddr
	// peer's public key if byzantine fault tolerance used, nil otherwise
	PubKey *rsa.PublicKey
}

type Config struct {
	// unique id for current node
	ID uint8
	// port for interconnections between nodes
	ProtoPort uint16
	// port for accepting request from client
	ServicePort uint16
	// peers addr
	Peers map[uint8]*PeerConfig
	// client public key
	ClientKey *rsa.PublicKey
	// private key for this node
	SelfKey *rsa.PrivateKey
	// maximum number of concurrent proposal this node can issue
	MaxPendingProposals uint32
	// quorum number for majority
	QuorumNumber uint8
	// timeout for prepare phase
	PrepareTimeout time.Duration
	// timeout for accept phase
	AcceptTimeout time.Duration
	// waiting timeout for acceptors
	AcceptorTimeout time.Duration
	// log level
	LogLevel int
	// log output
	LogOutput io.Writer
	// file dir
	FileDir string
	// digest header name
	DigestHeader string
	// data id header name
	DataHeader string
	// signature header includes signature from client
	SignatureHeader string
	// file buffer size in byte
	FileBufferSize uint
	// delay for checking new files
	CheckingDelay time.Duration
	// number n of first nth of committed queue elements that need to recover
	RecoverCommittedNumber uint
	// wether to support byzantine fault tolerance
	ByzantineFaultTolerance bool
	// number of faulty machines the system can tolerate, this number must
	// corresponds to QuorumNumber
	FaultyNumber uint8
	// directory of keys
	KeyDir string
}

func ParseConfig(file string) (*Config, error) {
	c, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}
	parsed := gjson.Parse(string(c))
	conf := &Config{}
	// ID
	if id := parsed.Get("ID"); !id.Exists() {
		return nil, errors.New("[ID] not found")
	} else {
		conf.ID = uint8(id.Uint())
	}
	// Proto port default is 10000
	if protoPort := parsed.Get("ProtoPort"); !protoPort.Exists() {
		conf.ProtoPort = 10000
	} else {
		conf.ProtoPort = uint16(protoPort.Uint())
	}
	// service port default is 8000
	if servicePort := parsed.Get("ServicePort"); !servicePort.Exists() {
		conf.ServicePort = 8000
	} else {
		conf.ServicePort = uint16(servicePort.Uint())
	}
	// max pending proposals default 100
	if maxPending := parsed.Get("MaxPendingProposals"); !maxPending.Exists() {
		conf.MaxPendingProposals = 100
	} else {
		conf.MaxPendingProposals = uint32(maxPending.Uint())
	}
	// quorum number has no default value
	if qn := parsed.Get("QuorumNumber"); !qn.Exists() {
		return nil, errors.New("QuorumNumber not found")
	} else {
		conf.QuorumNumber = uint8(qn.Uint())
	}
	// prepare timeout default 3s
	if pTimeout := parsed.Get("PrepareTimeout"); !pTimeout.Exists() {
		conf.PrepareTimeout = time.Second * 3
	} else {
		conf.PrepareTimeout = time.Second * time.Duration(pTimeout.Int())
	}
	// accept timeout default 3s
	if aTimeout := parsed.Get("AcceptTimeout"); !aTimeout.Exists() {
		conf.AcceptTimeout = time.Second * 3
	} else {
		conf.AcceptTimeout = time.Second * time.Duration(aTimeout.Int())
	}
	// acceptor timeout default 3s
	if apTimeout := parsed.Get("AcceptorTimeout"); !apTimeout.Exists() {
		conf.AcceptorTimeout = time.Second * 3
	} else {
		conf.AcceptorTimeout = time.Second * time.Duration(apTimeout.Int())
	}
	// log level default is ERROR
	if logLevel := parsed.Get("LogLevel"); !logLevel.Exists() {
		conf.LogLevel = ERROR
	} else {
		conf.LogLevel = int(logLevel.Int())
	}
	// log output default is stdout
	if logOut := parsed.Get("LogOutput"); !logOut.Exists() {
		conf.LogOutput = os.Stdout
	} else {
		if log, err := os.Open(logOut.String()); err != nil {
			return nil, errors.New("cannot open file " + logOut.String())
		} else {
			conf.LogOutput = log
		}
	}
	// file dir default "files"
	if fileDir := parsed.Get("FileDir"); !fileDir.Exists() {
		conf.FileDir = "files"
	} else {
		conf.FileDir = fileDir.String()
	}
	// headers must be set
	if diHead := parsed.Get("DigestHeader"); !diHead.Exists() {
		return nil, errors.New("DigestHeader not found")
	} else {
		conf.DigestHeader = diHead.String()
	}
	if daHead := parsed.Get("DataHeader"); !daHead.Exists() {
		return nil, errors.New("DataHeader not found")
	} else {
		conf.DataHeader = daHead.String()
	}
	if sHead := parsed.Get("SignatureHeader"); !sHead.Exists() {
		return nil, errors.New("SignatureHeader not found")
	} else {
		conf.SignatureHeader = sHead.String()
	}
	// File buf default 1024B
	if fb := parsed.Get("FileBufferSize"); !fb.Exists() {
		conf.FileBufferSize = 1024
	} else {
		conf.FileBufferSize = uint(fb.Uint())
	}
	// checking delay default 1s
	if cd := parsed.Get("CheckingDelay"); !cd.Exists() {
		conf.CheckingDelay = time.Second * 1
	} else {
		conf.CheckingDelay = time.Second * time.Duration(cd.Int())
	}
	// recover commit number default 5
	if rcn := parsed.Get("RecoverCommittedNumber"); !rcn.Exists() {
		conf.RecoverCommittedNumber = 5
	} else {
		conf.RecoverCommittedNumber = uint(rcn.Uint())
	}
	// Byzantine is not used by default
	if bft := parsed.Get("ByzantineFaultTolerance"); !bft.Exists() {
		conf.ByzantineFaultTolerance = false
	} else {
		conf.ByzantineFaultTolerance = bft.Bool()
	}
	// faulty number must not be empty if bft is true
	if fn := parsed.Get("FaultyNumber"); !fn.Exists() {
		if conf.ByzantineFaultTolerance {
			return nil, errors.New("FaultyNumber not found")
		}
	} else {
		conf.FaultyNumber = uint8(fn.Uint())
	}
	// key dir must not be empty if bft is true
	if kd := parsed.Get("KeyDir"); !kd.Exists() {
		if conf.ByzantineFaultTolerance {
			return nil, errors.New("KeyDir not found")
		}
	} else {
		conf.KeyDir = kd.String()
	}
	// client and self keys
	if ck := parsed.Get("ClientKey"); !ck.Exists() {
		return nil, errors.New("ClientKey not found")
	} else {
		if pk, err := ParsePublicKey(conf.KeyDir + "/" + ck.String()); err != nil {
			return nil, errors.New("failed to parse ClientKey")
		} else {
			conf.ClientKey = pk
		}
	}
	if sk := parsed.Get("SelfKey"); !sk.Exists() {
		return nil, errors.New("SelfKey not found")
	} else {
		if pk, err := ParsePrivateKey(conf.KeyDir + "/" + sk.String()); err != nil {
			return nil, errors.New("failed to parse SelfKey")
		} else {
			conf.SelfKey = pk
		}
	}
	// parse peers
	if peers := parsed.Get("Peers"); !peers.Exists() {
		return nil, errors.New("peers not fuond")
	} else {
		conf.Peers = make(map[uint8]*PeerConfig)
		succeed := true
		var e error
		peers.ForEach(func(key, value gjson.Result) bool {
			id := uint8(value.Get("ID").Uint())
			keypath := value.Get("PublicKey").String()
			addr := value.Get("Addr").String()
			// parse key and addr
			if pk, err := ParsePublicKey(conf.KeyDir + "/" + keypath); err != nil {
				succeed = false
				e = err
				return false
			} else {
				conf.Peers[id] = new(PeerConfig)
				conf.Peers[id].PubKey = pk
			}
			if tcpaddr, err := net.ResolveTCPAddr("tcp", addr); err != nil {
				succeed = false
				e = err
				return false
			} else {
				conf.Peers[id].Addr = *tcpaddr
			}
			return true
		})
		if !succeed {
			return nil, errors.New("failed to parse Peers: " + e.Error())
		}
	}
	return conf, nil
}
