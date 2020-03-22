package main

import (
	"encoding/json"
	"fmt"
)

// KBDev identifies unique peer by Keybase username and device name. Usernames
// are unique for all Keybase, device names are unique per user.
type KBDev struct {
	Username string `json:"username"`
	Device   string `json:"device"`
}

// KeybasePeer is a peer found in peers.txt, not necessarily a WireGuard peer
// (yet, or ever) - depending if we've heard their announcement.
type KeybasePeer struct {
	// Device in Keybase realm. Username+Devicename pair. Also found in
	// peers.txt. We will be looking for announcement messages in Keybase chat
	// from that device.
	Device KBDev `json:"device"`

	// IP address for the peer. If we hear an announcement from that peer, we
	// will give them this address.
	IP string `json:"ip"`

	// Wireguard public key
	PublicKey string `json:"public_key"`

	// MulticastID is randomized, announced by each peers, used for finding one
	// another in LAN. (TODO: Just a though. Of course it's not even started to
	// be implemented.)
	MulticastID string

	// Endpoint to reach the peer. Either from the announcement, on from LAN
	// discovery (LAN not implemented).
	Endpoint string

	LastAnnouncement string
}

type PeerJSON struct {
	Username string `json:"username"`
	Device   string `json:"device""`
	IP       string `json:"ip"`
}

func (p PeerJSON) GetKBDev() KBDev {
	return KBDev{
		Username: p.Username,
		Device:   p.Device,
	}
}

func LoadPeerList(mctx MetaContext) (peers []PeerJSON, err error) {
	peerBytes, err := KeybaseReadKBFS(mctx.API(), fmt.Sprintf("/keybase/team/%s/peers.txt", mctx.Prog.KeybaseTeam))
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(peerBytes, &peers)
	if err != nil {
		return nil, fmt.Errorf("Failed to unmarshal peers.txt: %w", err)
	}
	return peers, nil
}

// WireguardPeer is a struct to use when interacting with WireGuard through
// `wg` command.
type WireguardPeer struct {
	AllowedIP string
	Endpoint  string
	PublicKey string
}
