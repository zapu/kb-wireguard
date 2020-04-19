package kbwg

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"sync"

	"github.com/davecgh/go-spew/spew"
	"github.com/zapu/kb-wireguard/libpipe"
	"github.com/zapu/kb-wireguard/libwireguard"
	"gortc.io/stun"
)

func MakeMAC(msg string, key string) (ret string, err error) {
	keyBytes, err := hex.DecodeString(key)
	if err != nil {
		return ret, err
	}
	mac := hmac.New(sha256.New, keyBytes)
	mac.Write([]byte(msg))
	resultMac := mac.Sum(nil)
	ret = hex.EncodeToString(resultMac)
	return ret, nil
}

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

type EndpointID string

type EndpointCandidate struct {
	Hostport   libwireguard.HostPort
	Type       string
	EndpointID EndpointID
}

type EndpointCandidateProviderImpl struct{}

func GetLocalIPAddresses() (ret []net.IP, err error) {
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
			ip, mask, err := net.ParseCIDR(addr.String())
			if err != nil {
				return nil, err
			}
			_ = mask
			ret = append(ret, ip)
		}
	}
	return ret, nil
}

type ReflectedAddress struct {
	local     libwireguard.HostPort
	reflected libwireguard.HostPort
}

func GetSTUNAddresses(localAddrs []net.IP, port uint16) (ret []ReflectedAddress, err error) {
	raddr, err := net.ResolveUDPAddr("udp", "stun.l.google.com:19302")
	if err != nil {
		return nil, fmt.Errorf("error resolving remote addr: %w", err)
	}

	for _, addr := range localAddrs {
		var localStr string
		if addr.To4() != nil {
			localStr = fmt.Sprintf("%s:%d", addr.String(), port)
		} else {
			localStr = fmt.Sprintf("[%s]:%d", addr.String(), port)
		}
		log.Printf("STUNning from %s", localStr)
		laddr, err := net.ResolveUDPAddr("udp", localStr)
		if err != nil {
			return nil, fmt.Errorf("error resolve local upd addr: %s : %w", localStr, err)
		}
		conn, err := net.DialUDP("udp", laddr, raddr)
		if err != nil {
			err = fmt.Errorf("error dialing udp local: %s remote: %s : %w", laddr, raddr, err)
			log.Printf("Skipping stun attempt: %s", err)
			continue
		}
		defer func() {
			err := conn.Close()
			if err != nil {
				log.Printf("Failed to close socket %s: %s", laddr, err)
			}
		}()
		stunCli, err := stun.NewClient(conn)
		if err != nil {
			return nil, fmt.Errorf("error in stun.NewClient: %w", err)
		}
		message, err := stun.Build(stun.TransactionID, stun.BindingRequest)
		if err != nil {
			return nil, fmt.Errorf("error building stun tx: %w", err)
		}
		var doErr error
		err = stunCli.Do(message, func(res stun.Event) {
			if res.Error != nil {
				doErr = fmt.Errorf("error in stun.Do: %w", res.Error)
				return
			}
			var xorAddr stun.XORMappedAddress
			if err := xorAddr.GetFrom(res.Message); err != nil {
				doErr = fmt.Errorf("error decoding xor addr: %w", err)
				return
			}
			log.Printf("Found address: %s", xorAddr.String())

			ret = append(ret, ReflectedAddress{
				local: libwireguard.HostPort{
					Host: addr,
					Port: port,
				},
				reflected: libwireguard.HostPort{
					Host: xorAddr.IP,
					Port: uint16(xorAddr.Port),
				},
			})
		})
		if doErr != nil {
			return nil, doErr
		}
	}
	return ret, nil
}

type EndpointCandidateProvider interface {
	GetEndpointCandidates() ([]EndpointCandidate, error)
}

type DCRequestMsg struct {
	Recipient KBDev
	ReqID     DCRequestID
	Endpoints []string
}

type DCRequestID string

type EndpointCookie string

type DCResponseMsg struct {
	ReqID     DCRequestID
	Endpoints []string
}

type DirectConnectionID string

func MakeID() string {
	return hex.EncodeToString(RandBytes(16))
}

type DCBase struct {
	user  KBDev
	reqID DCRequestID

	localPort      uint16
	localAddrs     []net.IP
	reflectedAddrs []ReflectedAddress

	// Remote party endpoints.
	remoteAddrs []libwireguard.HostPort
}

func (dc *DCBase) InitNetwork() error {
	minPort := uint16(1000)
	port := uint16(rand.Intn(int(^uint16(0)-minPort)) + int(minPort))

	localAddrs, err := GetLocalIPAddresses()
	if err != nil {
		return err
	}

	refAddrs, err := GetSTUNAddresses(localAddrs, port)
	if err != nil {
		return err
	}

	dc.localPort = port
	dc.localAddrs = localAddrs
	dc.reflectedAddrs = refAddrs
	return nil
}

func (dc *DCBase) GetEndpointCandidates() (ret []libwireguard.HostPort) {
	set := make(map[string]bool, len(dc.localAddrs)+len(dc.reflectedAddrs))
	addAddr := func(addr libwireguard.HostPort) {
		addrStr := addr.String()
		if _, found := set[addrStr]; found {
			return
		}
		set[addrStr] = true
		if addr.Host.To4() == nil {
			// Skip IPv6 for now.
			// TODO: Fix parsing IPv6 so we can give them out as candidates.
			return
		}
		ret = append(ret, addr)
	}
	for _, v := range dc.localAddrs {
		addAddr(libwireguard.HostPort{
			Host: v,
			Port: dc.localPort,
		})
	}
	for _, v := range dc.reflectedAddrs {
		addAddr(v.reflected)
	}
	return ret
}

type DirectConnection struct {
	reqID  DCRequestID
	connID DirectConnectionID

	localAddrs     []net.IP
	reflectedAddrs []ReflectedAddress

	listeners []net.PacketConn
	conns     []net.UDPConn

	isOutgoing bool
}

type DCOutgoing struct {
	DCBase

	endpoints []libwireguard.HostPort
	conns     []net.PacketConn
}

func (dc *DCOutgoing) MakeRequestMsg() (msg DCRequestMsg) {
	endpoints := dc.GetEndpointCandidates()
	msg.Recipient = dc.user
	msg.ReqID = dc.reqID
	msg.Endpoints = make([]string, len(endpoints))
	for i, v := range endpoints {
		msg.Endpoints[i] = v.String()
	}
	return msg
}

func (dc *DirectConnection) close() {
	for _, conn := range dc.conns {
		conn.Close()
	}
}

type DCIncoming struct {
	DCBase

	// Each endpoint is getting a cookie so we can identify which one is
	// connectable.
	cookies map[string]EndpointCookie
}

type DirectConnectionMgr struct {
	sync.RWMutex

	outgoingConns map[DCRequestID]*DCOutgoing
	incomingConns map[DCRequestID]*DCIncoming
}

func MakeDCMgr() *DirectConnectionMgr {
	ret := &DirectConnectionMgr{
		outgoingConns: make(map[DCRequestID]*DCOutgoing),
		incomingConns: make(map[DCRequestID]*DCIncoming),
	}
	return ret
}

func (mgr *DirectConnectionMgr) MakeOutgoingConnection() (ret *DCOutgoing, err error) {
	mgr.Lock()
	defer mgr.Unlock()

	ret = &DCOutgoing{}
	ret.reqID = DCRequestID(MakeID())

	err = ret.InitNetwork()
	if err != nil {
		return nil, err
	}

	mgr.outgoingConns[ret.reqID] = ret

	// We only want to bind to each local addr once.
	// localSet := make(map[string]bool, len(refAddrs))

	// for _, addr := range refAddrs {
	// 	addrStr := addr.local.String()
	// 	if _, found := localSet[addrStr]; found {
	// 		// We are already binding to this addr
	// 		continue
	// 	}

	// 	localSet[addrStr] = true
	// 	listener, err := net.ListenPacket("udp", addrStr)
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// 	ret.listeners = append(ret.listeners, listener)
	// }

	return ret, nil
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

func (mgr *DirectConnectionMgr) handleRequest(msg DCRequestMsg) (err error) {
	if _, found := mgr.incomingConns[msg.ReqID]; found {
		return fmt.Errorf("Got a connection request with re-used request ID: %s", msg.ReqID)
	}

	mgr.Lock()
	defer mgr.Unlock()

	// Is this a response to an outgoing request?
	out, found := mgr.outgoingConns[msg.ReqID]
	if found {

	}

	inc := &DCIncoming{}
	inc.reqID = msg.ReqID
	inc.remoteAddrs = make([]libwireguard.HostPort, len(msg.Endpoints))
	for i, v := range msg.Endpoints {
		hp := libwireguard.ParseHostPort(v)
		if hp.Port == 0 {
			return fmt.Errorf("Failed to parse host port: %s", v)
		}
		inc.remoteAddrs[i] = hp
	}

	err = inc.InitNetwork()
	if err != nil {
		return err
	}

	candidates := inc.GetEndpointCandidates()
	// inc.cookies = make(map[string]EndpointCookie, len(candidates))
	// for _, v := range candidates {
	// 	inc.cookies[v.String()] = EndpointCookie(MakeID())
	// }

	responseMsg := DCRequestMsg{
		ReqID:     msg.ReqID,
		Endpoints: make([]string, len(candidates)),
	}
	for i, v := range candidates {
		responseMsg.Endpoints[i] = v.String()
	}

	responseBytes, _ := libpipe.SerializeMsgInterface("request", responseMsg)
	fmt.Printf("%s\n", responseBytes)

	spew.Dump(inc)

	return nil
}
