package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/zapu/kb-wireguard/kbwg"

	"github.com/keybase/go-keybase-chat-bot/kbchat"
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

func main() {
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

	prog := &kbwg.Program{}
	prog.KeybaseTeam = kbTeamArg
	prog.Endpoint = endpointArg

	var kbc *kbchat.API

	kbc, err := kbchat.Start(kbchat.RunOptions{})
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

	peers, err := kbwg.LoadPeerList(prog.MCtxTODO())
	if err != nil {
		fail("%s", err)
	}

	prog.KeybasePeers = make(map[kbwg.KBDev]kbwg.KeybasePeer, len(peers))

	var foundSelf bool
	for _, peer := range peers {
		kbPeer := kbwg.KeybasePeer{
			Device: peer.GetKBDev(),
			IP:     peer.IP,
		}

		if peer.Username == prog.Self.Username && peer.Device == prog.Self.Device {
			if foundSelf {
				// TODO: be smarter about finding duplicates in peers.txt
				fail("Found self twice???")
			}
			foundSelf = true
			prog.SelfPeer = kbPeer
			prog.KeybasePeers[kbPeer.Device] = kbPeer // TODO: temporary
		} else {
			prog.KeybasePeers[kbPeer.Device] = kbPeer
		}
	}

	fmt.Printf(":: Found total %d peers in peers.txt\n", len(prog.KeybasePeers))

	err = kbwg.FindAnnouncements(prog.MCtxTODO())
	if err != nil {
		fail("%s", err)
	}

	if !foundSelf {
		fail("Failed to find us in peers.txt. Looking for device: %q", prog.Self.Device)
	}

	// SendAnnouncement(prog.MCtxTODO())
}
