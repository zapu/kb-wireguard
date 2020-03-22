package kbwg

import (
	"encoding/json"
	"fmt"

	"github.com/keybase/go-keybase-chat-bot/kbchat"
)

type StatusJSONPart struct {
	Username string `json:"Username"`
	Device   struct {
		Type     string `json:"type"`
		Name     string `json:"name"`
		DeviceID string `json:"deviceID"`
	} `json:"Device"`
}

func KeybaseGetLoggedInStatus(api *kbchat.API) (ret StatusJSONPart, err error) {
	statusCmd := api.Command("status", "--json")
	statusBytes, err := statusCmd.Output()
	if err != nil {
		return ret, fmt.Errorf("Failed to run `keybase status --json`: %w", err)
	}

	var statusPart StatusJSONPart
	err = json.Unmarshal(statusBytes, &statusPart)
	if err != nil {
		return ret, fmt.Errorf("Failed to unmarshal status output: %w", err)
	}
	return statusPart, nil
}

func KeybaseReadKBFS(api *kbchat.API, path string) (contents []byte, err error) {
	cmd := api.Command("fs", "read", path)
	outBytes, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("Failed to run `keybase fs read` for %q: %w", path, err)
	}
	return outBytes, nil
}
