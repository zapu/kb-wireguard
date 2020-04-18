package kbwg

import (
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"sync"

	"github.com/zapu/kb-wireguard/libpipe"
	"github.com/zapu/kb-wireguard/libwireguard"
)

type PeerConnection struct {
	laddr net.Addr
	conn  net.PacketConn
}

func (p *PeerConnection) listenLoop() {
	buf := make([]byte, 65535)
	for {
		n, fromAddr, err := p.conn.ReadFrom(buf)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to read from udp: %s", err)
			continue
		}

		fmt.Printf(":: Read %d bytes on %s from %s", n, p.laddr, fromAddr)
	}
}

func StartListening() (ret PeerConnection, err error) {
	listener, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		return ret, err
	}

	ret.laddr = listener.LocalAddr()
	ret.conn = listener
	go ret.listenLoop()
	return ret, nil
}

type EndpointCandidate struct {
	Hostport libwireguard.HostPort
	Type     string
}

type EndpointCandidateProviderImpl struct{}

func (p *EndpointCandidateProviderImpl) GetEndpointCandidates() (ret []EndpointCandidate, err error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	for _, iface := range ifaces {
		if (iface.Flags & net.FlagLoopback) != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			_ = addr
			// ret = append(ret, addr.String())
		}
	}
	return ret, nil
}

type EndpointCandidateProvider interface {
	GetEndpointCandidates() ([]EndpointCandidate, error)
}

type DCRequestMsg struct {
	// List of endpoints that connection recipient will punch out to, to
	// condition its NAT.
	ID          DCRequestID
	MyEndpoints []string
}

type DCRequestID string

func MakeDCRequestID() DCRequestID {
	return DCRequestID(hex.EncodeToString(RandBytes(16)))
}

type DCIncoming struct {
	endpoints []libwireguard.HostPort
	laddr     net.Addr
	conn      net.PacketConn
}

type DirectConnectionMgr struct {
	sync.RWMutex

	outgoingConns map[DirectConnectionID]*DirectConnection

	incomingConns map[DCRequestID]*DCIncoming
}

func MakeDCMgr() *DirectConnectionMgr {
	ret := &DirectConnectionMgr{
		outgoingConns: make(map[DirectConnectionID]*DirectConnection),
		incomingConns: make(map[DCRequestID]*DCIncoming),
	}
	return ret
}

func (mgr *DirectConnectionMgr) OnMessage(rawMsg libpipe.PipeMsg) error {
	switch rawMsg.ID {
	case "request":
		var msg DCRequestMsg
		err := rawMsg.DeserializePayload(&msg)
		if err != nil {
			return err
		}
		return mgr.handleRequest(msg)
	}
	return nil
}

func (mgr *DirectConnectionMgr) handleRequest(msg DCRequestMsg) error {
	if _, found := mgr.incomingConns[msg.ID]; found {
		return fmt.Errorf("Got a connection request with re-used request ID: %s", msg.ID)
	}

	inc := &DCIncoming{}
	inc.endpoints = make([]libwireguard.HostPort, len(msg.MyEndpoints))
	for i, v := range msg.MyEndpoints {
		hp := libwireguard.ParseHostPort(v)
		if hp.Port == 0 {
			return fmt.Errorf("Failed to parse host port: %s", v)
		}
		inc.endpoints[i] = hp
	}

	return nil
}

type DirectConnectionID string
type DirectConnection struct {
	ID DirectConnectionID
}

func AllocateDirectConnection() *DirectConnection {
	ret := &DirectConnection{
		ID: DirectConnectionID(hex.EncodeToString(RandBytes(16))),
	}
	return ret
}
