package devowner

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
	PublicKey  WireguardPubKey
	AllowedIPs string
	Endpoint   string

	Label string
}

func SerializeConfig(conf WireguardConfig) string {
	var builder strings.Builder
	builder.WriteString("[Interface]\n")
	builder.WriteString(fmt.Sprintf("ListenPort = %d\n", conf.ListenPort))
	builder.WriteString(fmt.Sprintf("PrivateKey = %s\n", conf.PrivateKey))
	builder.WriteString("\n")

	for _, peer := range conf.Peers {
		builder.WriteString("[Peer]\n")
		if peer.Label != "" {
			builder.WriteString(fmt.Sprintf("# %s\n", peer.Label))
		}
		builder.WriteString(fmt.Sprintf("PublicKey = %s\n", peer.PublicKey))
		builder.WriteString(fmt.Sprintf("AllowedIPs = %s\n", peer.AllowedIPs))
		builder.WriteString(fmt.Sprintf("Endpoint = %s\n", peer.Endpoint))
		builder.WriteString("\n")
	}

	return builder.String()
}
