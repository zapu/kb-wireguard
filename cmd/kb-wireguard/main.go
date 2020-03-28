package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/davecgh/go-spew/spew"
	"github.com/zapu/kb-wireguard/kbwg"
	"github.com/zapu/kb-wireguard/libwireguard"

	"github.com/keybase/go-keybase-chat-bot/kbchat"

	"gortc.io/stun"
)

func fail(msg string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, msg+"\n", args...)
	os.Exit(3)
}

func failUsage(msg string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "Error: "+msg+"\n\n", args...)
	flag.Usage()
	os.Exit(2)
}

// WIP
func stunDial() {
	fmt.Printf(":: Trying to STUN\n")
	// Creating a "connection" to STUN server.
	c, err := stun.Dial("udp", "stun.l.google.com:19302")
	if err != nil {
		panic(err)
	}
	// Building binding request with random transaction id.
	message := stun.MustBuild(stun.TransactionID, stun.BindingRequest)
	// Sending request to STUN server, waiting for response message.
	if err := c.Do(message, func(res stun.Event) {
		if res.Error != nil {
			panic(res.Error)
		}
		// Decoding XOR-MAPPED-ADDRESS attribute from message.
		var xorAddr stun.XORMappedAddress
		if err := xorAddr.GetFrom(res.Message); err != nil {
			panic(err)
		}
		spew.Dump(xorAddr)
	}); err != nil {
		panic(err)
	}
}

func main() {
	var err error

	// stunDial()

	var endpointArg string
	var kbTeamArg string
	flag.StringVar(&endpointArg, "endpoint", "", "Public endpoint for this machine. Will be announced to other peers.")
	flag.StringVar(&kbTeamArg, "team", "", "Keybase team name to coordinate peering with. Each team can be considered a separate VPN where team members can connect to each other.")
	flag.Parse()

	if endpointArg == "" {
		failUsage("`endpoint` argument is required")
	}
	if kbTeamArg == "" {
		failUsage("`team` argument is required")
	}

	endpointHostPortArg := libwireguard.ParseHostPort(endpointArg)
	if endpointHostPortArg.IsNil() {
		failUsage("`endpoint` argument has to be host:port")
	}

	prog := &kbwg.Program{}
	prog.KeybaseTeam = kbTeamArg
	prog.Endpoint = endpointHostPortArg

	var kbc *kbchat.API

	kbc, err = kbchat.Start(kbchat.RunOptions{})
	if err != nil {
		fail("Failed to start kbchat: %s", err)
	}

	fmt.Printf(":: Started Keybase Chat API\n")

	prog.API = kbc

	// fmt.Printf("My username is: %s\n", kbc.GetUsername())

	// res, err := kbc.ListChannels("wgtestteam1")
	// if err != nil {
	// 	fail("%s", err)
	// }
	// spew.Dump(res)

	err = prog.LoadSelf(context.TODO())
	if err != nil {
		fail("%s", err)
	}

	fmt.Printf(":: We are logged in as: %s (%s)\n", prog.Self.Username, prog.Self.Device)
	fmt.Printf(":: Trying to peer with team @%s\n", prog.KeybaseTeam)

	announceConv, err := kbwg.AnnounceFindChat(prog.MCtxTODO())
	if err != nil {
		fail("didn't find announce conv: %s", err)
	}
	prog.AnnounceChannel = announceConv.Channel

	fmt.Printf(":: Found announcement channel: @%s#%s\n", announceConv.Channel.Name, announceConv.Channel.TopicName)

	// Load peers
	peers, err := kbwg.LoadPeerList(prog.MCtxTODO())
	if err != nil {
		fail("%s", err)
	}

	prog.KeybasePeers = make(map[kbwg.KBDev]kbwg.KeybasePeer, len(peers))

	var foundSelf bool
	for _, peer := range peers {
		kbPeer, err := peer.MakeKeybasePeer()
		if err != nil {
			fail("failed to parse kb peer %v %s:", peer.GetKBDev(), err)
		}

		if peer.Username == prog.Self.Username && peer.Device == prog.Self.Device {
			if foundSelf {
				// TODO: be smarter about finding duplicates in peers.json
				fail("Found self twice???")
			}
			foundSelf = true
			prog.SelfPeer = kbPeer
		} else {
			prog.KeybasePeers[kbPeer.Device] = kbPeer
		}
	}

	if !foundSelf {
		fail("Failed to find us in peers.json. Maybe we can't peer with this team. Looking for device: %q", prog.Self.Device)
	}

	fmt.Printf(":: We are: %s\n", prog.SelfPeer.IP)
	fmt.Printf(":: Found %d other peer(s) in peers.json\n", len(prog.KeybasePeers))

	fmt.Printf(":: Trying to start WireGuard device... You may be asked for `sudo` password.\n")

	devRun, err := kbwg.RunDevRunner(prog.SelfPeer.IP.String())
	if err != nil {
		fail("Failed to run dev owner: %s", err)
	}

	wgPubKey := <-devRun.PubKeyCh
	prog.SelfPeer.PublicKey = wgPubKey

	prog.DevRunner = devRun

	go kbwg.AnnouncementsBgTask(prog.MCtxTODO())
	go kbwg.SelfAnnouncementBgTask(prog.MCtxTODO())

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

loop:
	for {
		select {
		case <-sigs:
			fmt.Printf("! Stopping on signal...\n")
			break loop
		}
	}

	devRun.Process.Wait()

	fmt.Printf(":: kb-wireguard exiting...\n")

	os.Exit(0)
}
