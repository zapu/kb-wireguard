package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/zapu/kb-wireguard/kbwg"
	"github.com/zapu/kb-wireguard/libpipe"
)

func main() {
	rand.Seed(time.Now().UnixNano())
	log.Print("Hello world")

	mgr := kbwg.MakeDCMgr()
	conn, err := mgr.MakeOutgoingConnection()
	if err != nil {
		panic(err)
	}

	candidates := conn.GetEndpointCandidates()
	spew.Dump(candidates)

	reqMsg := conn.MakeRequestMsg()
	reqMsgBytes, _ := libpipe.SerializeMsgInterface("request", reqMsg)
	fmt.Printf("%s\n", string(reqMsgBytes))

	lineReader := bufio.NewReader(os.Stdin)
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
