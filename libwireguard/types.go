package libwireguard

type WireguardPrivKey string

func (pk WireguardPrivKey) String() string {
	return "<private key>"
}

type WireguardPubKey string
