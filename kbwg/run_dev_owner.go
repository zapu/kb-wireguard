package kbwg

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"

	devowner "github.com/zapu/kb-wireguard/devowner"
)

// Run process that sets up and "owns" the wireguard device, and also tears it
// down when it exits.

type DevRunnerProcess struct {
	doneCh chan struct{}
}

func RunDevRunner() {
	var process DevRunnerProcess
	process.doneCh = make(chan struct{})

	cmd := exec.Command("sudo", "./devowner")
	stdout, _ := cmd.StdoutPipe()
	stdoutReader := bufio.NewReader(stdout)

	stderr, _ := cmd.StderrPipe()
	stderrReader := bufio.NewReader(stderr)

	go func() {
		for {
			line, err := stdoutReader.ReadBytes('\n')
			if err != nil || len(line) == 0 {
				continue
			}
			var msg devowner.PipeMsg
			err = json.Unmarshal(line, &msg)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to unmarshall from RunDev: %s", err)
				continue
			}
			err = handleDevRunnerControlMsg(msg)
			if err != nil {

			}
		}
	}()

	go func() {
		for {
			line, err := stderrReader.ReadBytes('\n')
			if err != nil || len(line) == 0 {
				continue
			}
			fmt.Printf("RunDev: %s\n", line)

			// TODO: Push these through channel as well
		}
	}()

	go cmd.Run()
}

func handleDevRunnerControlMsg(msg devowner.PipeMsg) error {
	if msg.ID == "pubkey" {
		pubkey := msg.Payload.(string)
		fmt.Printf("Received pub key from device runner: %s", pubkey)
	}
	return nil
}
