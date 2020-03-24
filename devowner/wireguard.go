package devowner

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

type WireguardPrivKey string
type WireguardPubKey string

func WireguardGenKey() (priv WireguardPrivKey, pub WireguardPubKey, err error) {
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
	return WireguardPrivKey(strings.TrimSpace(string(privBytes))),
		WireguardPubKey(strings.TrimSpace(string(pubBytes))),
		nil
}
