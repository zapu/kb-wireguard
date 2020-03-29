package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/zapu/kb-wireguard/devowner"
	"github.com/zapu/kb-wireguard/libpipe"
	"github.com/zapu/kb-wireguard/libwireguard"
)

/*
ip link add dev kbwg0 type wireguard

ip address add dev kbwg0 101.0.0.1/24



keys:
wg genkey
wg pubkey < $(priv)



conf:

wg setconf kbwg0 kbwg0.conf

wg syncconf kbwg0 kbwg0.conf


*/

const deviceName = "kbwg0"

func sudoExec(name string, args ...string) ([]byte, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(name, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		cmdStr := fmt.Sprintf("%s %s", name, strings.Join(args, " "))
		fmt.Fprintf(os.Stderr, "Command %q stderr:\n%s\n", cmdStr, stderr.String())
		return nil, fmt.Errorf("exec %q: %w", cmdStr, err)
	}
	return stdout.Bytes(), nil
}

func fail(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(3)
}

func serializeToStdout(id string, payload interface{}) {
	msg, err := libpipe.SerializeMsgInterface(id, payload)
	if err != nil {
		fail("libpipe fail: %s", err)
	}
	fmt.Printf("%s\n", msg)
}

func debug(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}

func messageReaderTask(ctx context.Context, pipeFilename string, ch chan libpipe.PipeMsg) error {
	fd, err := os.OpenFile(pipeFilename, os.O_RDONLY, os.ModeNamedPipe)
	if err != nil {
		return err
	}

	defer fd.Close()

	stdinReader := bufio.NewReader(fd)

	debug("Opened read side of pipe %s", pipeFilename)

	for {
		line, err := stdinReader.ReadBytes('\n')
		if err != nil {
			return err
		}
		var pipeMsg libpipe.PipeMsg
		err = json.Unmarshal(line, &pipeMsg)
		if err != nil {
			return err
		}
		err = ctx.Err()
		if err != nil {
			return err
		}
		ch <- pipeMsg
	}
}

type DeviceOwnerProgram struct {
	IPAddress string

	ConfigFilename string
	Config         libwireguard.WireguardConfig

	signals chan os.Signal
	msgCh   chan libpipe.PipeMsg
}

func (prog *DeviceOwnerProgram) mainLoop() {
	for {
		select {
		case <-prog.signals:
			debug("Stopping on signal...")
			return
		case msg := <-prog.msgCh:
			debug("Got msg: %s %d", string(msg.Payload), len(string(msg.Payload)))
			if msg.ID == "peers" {
				err := prog.handlePeersMessage(msg)
				if err != nil {
					debug("Failed to handler peers msg:", err)
				}
			}
		}
	}
}

func (prog *DeviceOwnerProgram) handlePeersMessage(msg libpipe.PipeMsg) error {
	var newPeers []libwireguard.WireguardPeer
	err := json.Unmarshal([]byte(msg.Payload), &newPeers)
	if err != nil {
		return fmt.Errorf("failed to unmarshal: %w", err)
	}
	prog.Config.Peers = newPeers
	return prog.flushConfig()
}

func (prog *DeviceOwnerProgram) flushConfig() error {
	cfgFile, err := os.OpenFile(prog.ConfigFilename, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	if _, err := cfgFile.Write([]byte(libwireguard.SerializeConfig(prog.Config))); err != nil {
		return fmt.Errorf("failed to Write: %w", err)
	}

	cfgFile.WriteString(fmt.Sprintf("\n# Config edited at: %s\n", time.Now().Local()))

	if err := cfgFile.Close(); err != nil {
		return fmt.Errorf("failed to Close: %w", err)
	}

	_, err = sudoExec("wg", "syncconf", "kbwg0", prog.ConfigFilename)
	if err != nil {
		return fmt.Errorf("failed to 'wg syncconf': %w", err)
	}

	debug("syncconf successful")
	return nil
}

func main() {
	debug(`Hello from device runner ("run-dev"): %d %d`, os.Getuid(), os.Geteuid())
	if os.Getuid() != 0 {
		fail("Needs to run as root to control wireguard...")
	}

	var pipeFilename string
	var initialIPAddress string
	flag.StringVar(&pipeFilename, "pipe", "", "")
	flag.StringVar(&initialIPAddress, "ip", "", "")
	flag.Parse()

	privKey, pubKey, err := devowner.WireguardGenKey()
	if err != nil {
		fail("%s", err)
	}

	debug(":: Priv key: %s", privKey)
	debug(":: Pub key: %s", pubKey)

	serializeToStdout("pubkey", pubKey)

	prog := &DeviceOwnerProgram{}

	var conf libwireguard.WireguardConfig
	conf.PrivateKey = privKey
	conf.ListenPort = 51820

	prog.Config = conf

	// The moment we start doing things that need cleanups, start handling
	// signals.
	prog.signals = make(chan os.Signal, 1)
	signal.Notify(prog.signals, syscall.SIGINT, syscall.SIGTERM)

	tmpfile, err := ioutil.TempFile("", fmt.Sprintf("%s.*.conf", deviceName))
	if err != nil {
		fail("%s", err)
	}

	prog.ConfigFilename = tmpfile.Name()

	debug(":: Config filename: %s", tmpfile.Name())
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(libwireguard.SerializeConfig(conf))); err != nil {
		fail("%s", err)
	}
	if err := tmpfile.Close(); err != nil {
		fail("%s", err)
	}

	debug("Setting up device %s", deviceName)

	_, err = sudoExec("ip", "link", "add", "dev", deviceName, "type", "wireguard")
	if err != nil {
		fail("%s", err)
	}

	_, err = sudoExec("wg", "setconf", "kbwg0", tmpfile.Name())
	if err != nil {
		// fail("%s", err)
		debug("Failed to setconf: %s", err)
	}

	if initialIPAddress != "" {
		ipAddr := initialIPAddress + "/24"
		_, err = sudoExec("ip", "address", "add", "dev", deviceName, ipAddr)
		if err != nil {
			debug("Failed to set ip: %s", err)
		} else {
			debug("Set ip address to %s", ipAddr)
		}

		_, err = sudoExec("ip", "link", "set", "up", "dev", deviceName)
		if err != nil {
			debug("failed to bring the interface up: %s", err)
		}
	} else {
		debug("-ip flag not provided, not setting ip address")
	}

	prog.msgCh = make(chan libpipe.PipeMsg)
	readCtx, cancelRead := context.WithCancel(context.Background())
	if pipeFilename != "" {
		go func() {
			err := messageReaderTask(readCtx, pipeFilename, prog.msgCh)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error from messageReaderTask: %s\n", err)
			}
		}()
	} else {
		debug("Pipe filename not provided - no messages will be received, but continuing anyway.")
	}

	prog.mainLoop()

	cancelRead()
	debug("Removing device %s", deviceName)

	_, err = sudoExec("ip", "link", "delete", "dev", deviceName)
	if err != nil {
		fail("%s", err)
	}

	debug("Device removed... exiting")
}
