package kbwg

import (
	"encoding/json"
	"fmt"

	"github.com/zapu/kb-wireguard/libwireguard"
)

// KBDev identifies unique peer by Keybase username and device name. Usernames
// are unique for all Keybase, device names are unique per user.
type KBDev struct {
	Username string `json:"username"`
	Device   string `json:"device"`
}

// KeybasePeer is a peer found in peers.json, not necessarily a WireGuard peer
// (yet, or ever) - depending if we've heard their announcement.
type KeybasePeer struct {
	// Device in Keybase realm. Username+Devicename pair. Also found in
	// peers.json. We will be looking for announcement messages in Keybase chat
	// from that device.
	Device KBDev `json:"device"`

	// Was there an announcement from that peer?
	Active bool

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

	LastAnnouncement AnnounceMsg
}

type PeerJSON struct {
	Username string `json:"username"`
	Device   string `json:"device"`
	IP       string `json:"ip"`
}

func (p PeerJSON) GetKBDev() KBDev {
	return KBDev{
		Username: p.Username,
		Device:   p.Device,
	}
}

func LoadPeerList(mctx MetaContext) (peers []PeerJSON, err error) {
	peerBytes, err := KeybaseReadKBFS(mctx.API(), fmt.Sprintf("/keybase/team/%s/peers.json", mctx.Prog.KeybaseTeam))
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(peerBytes, &peers)
	if err != nil {
		return nil, fmt.Errorf("Failed to unmarshal peers.json: %w", err)
	}
	return peers, nil
}

func SerializeWireGuardPeerList(mctx MetaContext) (ret []libwireguard.WireguardPeer) {
	ret = make([]libwireguard.WireguardPeer, 0, len(mctx.Prog.KeybasePeers))
	for _, v := range mctx.Prog.KeybasePeers {
		if !v.Active {
			continue
		}

		label := fmt.Sprintf("%s (%s)", v.Device.Username, v.Device.Device)
		ret = append(ret, libwireguard.WireguardPeer{
			PublicKey:  libwireguard.WireguardPubKey(v.PublicKey),
			AllowedIPs: v.IP,
			Endpoint:   v.Endpoint,
			Label:      label,
		})
	}
	return ret
}
