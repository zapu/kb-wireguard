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
	"time"

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

func MakeID() string {
	return hex.EncodeToString(RandBytes(16))
}

type DCBase struct {
	user  KBDev
	reqID DCRequestID

	localPort      uint16
	localAddrs     []net.IP
	reflectedAddrs []ReflectedAddress
	localEndpoints []libwireguard.HostPort

	// Remote party endpoints.
	remoteAddrs []libwireguard.HostPort

	listener *net.UDPConn
	conns    []*net.UDPConn

	receivedProbe bool
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

func (dc *DCBase) StartListeningForProbes() error {
	listener, err := net.ListenUDP("udp", &net.UDPAddr{
		Port: int(dc.localPort),
		IP:   net.IPv4zero,
	})
	if err != nil {
		return err
	}

	dc.listener = listener
	go func() {
		buf := make([]byte, 64)
		for {
			n, raddr, err := dc.listener.ReadFrom(buf)
			if err != nil {
				log.Printf("Probe listener: read error: %s", err)
				break
			}
			log.Printf("Got probe at %d from %s: %q", dc.localPort, raddr.String(), hex.EncodeToString(buf[:n]))
		}
	}()
	return nil
}

func (dc *DCBase) StartProbing() error {
	// for _, v := range dc.remoteAddrs {
	// 	raddr := &net.UDPAddr{
	// 		IP:   v.Host,
	// 		Port: int(v.Port),
	// 	}
	// 	conn, err := net.DialUDP("udp", laddr, raddr)
	// 	if err != nil {
	// 		return fmt.Errorf("Failed to dial udp in StartProbing: %s -> %s: %w",
	// 			laddr.String(), raddr.String(), err)
	// 	}
	// 	dc.conns = append(dc.conns, conn)
	// }

	go func() {
		for {
			for _, v := range dc.remoteAddrs {
				raddr := &net.UDPAddr{
					IP:   v.Host,
					Port: int(v.Port),
				}
				log.Printf("Probing %s...", raddr.String())
				_, err := dc.listener.WriteTo([]byte("AAAA"), raddr)
				if err != nil {
					log.Printf("Probe writer: write error: %s", err)
				}
			}
			time.Sleep(1 * time.Second)
		}
	}()
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

type DirectConnectionMgr struct {
	sync.RWMutex

	connections map[DCRequestID]*DCBase
}

func MakeDCMgr() *DirectConnectionMgr {
	ret := &DirectConnectionMgr{
		connections: make(map[DCRequestID]*DCBase),
	}
	return ret
}

func (mgr *DirectConnectionMgr) MakeOutgoingConnection() (ret *DCBase, err error) {
	mgr.Lock()
	defer mgr.Unlock()

	ret, err = mgr.makeConnection(DCRequestID(MakeID()))
	if err != nil {
		return nil, err
	}

	candidates := ret.GetEndpointCandidates()
	response := DCRequestMsg{
		ReqID:     ret.reqID,
		Endpoints: make([]string, len(candidates)),
	}
	for i, v := range candidates {
		response.Endpoints[i] = v.String()
	}
	msgBytes, _ := libpipe.SerializeMsgInterface("request", response)
	fmt.Printf("%s\n", string(msgBytes))
	return ret, nil
}

func (mgr *DirectConnectionMgr) makeConnection(reqID DCRequestID) (ret *DCBase, err error) {
	ret = &DCBase{
		reqID: reqID,
	}

	err = ret.InitNetwork()
	if err != nil {
		return nil, err
	}

	mgr.connections[ret.reqID] = ret

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

func (mgr *DirectConnectionMgr) handleRequest(msg DCRequestMsg) error {
	mgr.Lock()
	defer mgr.Unlock()

	dc, found := mgr.connections[msg.ReqID]
	if found {
		log.Printf("Got a response to conn %s", dc.reqID)
		// This is something that we are already handling (so it's a response
		// to a connection we are trying to establish).
		dc.remoteAddrs = make([]libwireguard.HostPort, len(msg.Endpoints))
		for i, v := range msg.Endpoints {
			hp := libwireguard.ParseHostPort(v)
			if hp.IsNil() {
				return fmt.Errorf("Failed to parse remote endpoint %q", v)
			}
			dc.remoteAddrs[i] = hp
		}
		err := dc.StartListeningForProbes()
		if err != nil {
			return err
		}
		err = dc.StartProbing()
		if err != nil {
			return err
		}
	} else {
		// Someone else is trying to establish connection to us.
		dc, err := mgr.makeConnection(msg.ReqID)
		if err != nil {
			return err
		}
		candidates := dc.GetEndpointCandidates()
		response := DCRequestMsg{
			ReqID:     msg.ReqID,
			Endpoints: make([]string, len(candidates)),
		}
		for i, v := range candidates {
			response.Endpoints[i] = v.String()
		}
		msgBytes, _ := libpipe.SerializeMsgInterface("request", response)
		fmt.Printf("%s\n", string(msgBytes))

		err = dc.StartListeningForProbes()
		if err != nil {
			return err
		}
		err = dc.StartProbing()
		if err != nil {
			return err
		}
	}

	return nil
}
