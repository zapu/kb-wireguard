package main

import (
	"bufio"
	"encoding/json"
	"log"
	"net"
	"os"
	"strings"

	"github.com/davecgh/go-spew/spew"
	"github.com/zapu/kb-wireguard/kbwg"
	"github.com/zapu/kb-wireguard/libpipe"
)

func main() {
	log.Print("Hello world")

	ifaces, err := net.Interfaces()
	if err != nil {
		panic(err)
	}
	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil {
			log.Printf("Failed to get addresses for %s: %s", iface.Name, err)
			continue
		}
		spew.Dump(addrs)
	}

	lineReader := bufio.NewReader(os.Stdin)

	mgr := kbwg.MakeDCMgr()
	for {
		line, err := lineReader.ReadString('\n')
		if err != nil {
			panic(err)
		}
		line = strings.TrimSpace(line)
		if len(line) == 0 {
			continue
		}

		var pipeMsg libpipe.PipeMsg
		err = json.Unmarshal([]byte(line), &pipeMsg)
		if err != nil {
			log.Printf("Failed to unmarshal JSON: %s", err)
		}
		err = mgr.OnMessage(pipeMsg)
		if err != nil {
			log.Printf("Failed to handle message ID: %s, %s", pipeMsg.ID, err)
		}
	}
}
