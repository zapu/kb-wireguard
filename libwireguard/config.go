package libwireguard

import (
	"fmt"
	"strings"
)

type WireguardConfig struct {
	ListenPort uint16
	PrivateKey WireguardPrivKey

	Peers []WireguardPeer
}

type WireguardPeer struct {
	// NOTE: These are strings instead of complex types like net.IP or HostPort
	// because we are sending these through pipe in JSON format. net.IP sent in
	// JSON would become a base64 buffer and we want to keep things readable
	// (debuggable) for now.

	PublicKey           string
	AllowedIPs          string
	Endpoint            string
	PersistentKeepalive int

	Label string
}

func SerializeConfig(conf WireguardConfig) string {
	var builder strings.Builder
	builder.WriteString("[Interface]\n")
	builder.WriteString(fmt.Sprintf("ListenPort = %d\n", conf.ListenPort))
	builder.WriteString(fmt.Sprintf("PrivateKey = %s\n", string(conf.PrivateKey)))
	builder.WriteString("\n")

	for _, peer := range conf.Peers {
		builder.WriteString("[Peer]\n")
		if peer.Label != "" {
			builder.WriteString(fmt.Sprintf("# %s\n", peer.Label))
		}
		builder.WriteString(fmt.Sprintf("PublicKey = %s\n", peer.PublicKey))
		builder.WriteString(fmt.Sprintf("AllowedIPs = %s\n", peer.AllowedIPs))
		builder.WriteString(fmt.Sprintf("Endpoint = %s\n", peer.Endpoint))
		if peer.PersistentKeepalive != 0 {
			builder.WriteString(fmt.Sprintf("PersistentKeepalive = %d\n", peer.PersistentKeepalive))
		}
		builder.WriteString("\n")
	}

	return builder.String()
}
