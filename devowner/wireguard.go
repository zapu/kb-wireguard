package devowner

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	"github.com/zapu/kb-wireguard/libwireguard"
)

// WireguardGenKey calls `wg genkey` and `wg pubkey` to generate keypair.
func WireguardGenKey() (priv libwireguard.WireguardPrivKey, pub libwireguard.WireguardPubKey, err error) {
	cmd := exec.Command("wg", "genkey")
	privBytes, err := cmd.Output()
	if err != nil {
		return "", "", fmt.Errorf("Failed to run genkey: %w", err)
	}
	cmd = exec.Command("wg", "pubkey")
	cmd.Stdin = bytes.NewBuffer(privBytes)
	pubBytes, err := cmd.Output()
	if err != nil {
		return "", "", fmt.Errorf("Failed to run pubkey: %w", err)
	}
	// Need to trim output because it ends with newlines.
	return libwireguard.WireguardPrivKey(strings.TrimSpace(string(privBytes))),
		libwireguard.WireguardPubKey(strings.TrimSpace(string(pubBytes))),
		nil
}
