package kbwg

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"

	"github.com/davecgh/go-spew/spew"

	devowner "github.com/zapu/kb-wireguard/devowner"
)

type DevOwnerProcess struct {
	doneCh chan struct{}
}

func RunDevOwnerProcess() {
	var process DevOwnerProcess
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
			}
			spew.Dump(msg)
		}
	}()

	go func() {
		for {
			line, err := stderrReader.ReadBytes('\n')
			if err != nil || len(line) == 0 {
				continue
			}
			fmt.Printf("RunDev: %s\n", line)
		}
	}()

	go cmd.Run()
}
