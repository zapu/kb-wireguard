package main

import (
	"fmt"
	"math/rand"
	"net"
	"os"
	"strings"
	"time"

	"github.com/davecgh/go-spew/spew"
	"gortc.io/stun"
)

func failOnErr(err error, msg string) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", msg, err)
		os.Exit(2)
	}
}

func chatLoop(laddr *net.UDPAddr) {
	conn, err := net.ListenUDP("udp", laddr)
	failOnErr(err, "failed to ListenUDP")
	fmt.Printf("Listening on %s\n", laddr)

	defer conn.Close()

	buffer := make([]byte, 32*1024)
	for {
		n, from, err := conn.ReadFromUDP(buffer)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed ReadFromUDP: %s\n", err)
			continue
		}
		fmt.Printf("RECV from %s: %s\n", from, strings.TrimSpace(string(buffer[:n])))
	}
}

func main() {
	// randomize port
	rand.Seed(time.Now().UnixNano())
	minPort := uint16(1000)
	port := rand.Intn(int(^uint16(0)-minPort)) + int(minPort)
	fmt.Printf("Randomized port: %d\n", port)

	laddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("0.0.0.0:%d", port))
	failOnErr(err, "failed to resolve local address")

	spew.Dump(laddr)

	raddr, err := net.ResolveUDPAddr("udp", "stun.l.google.com:19302")
	failOnErr(err, "failed to resolve remote address")

	conn, err := net.DialUDP("udp", laddr, raddr)
	failOnErr(err, "failed to dial udp")

	fmt.Printf(":: Trying to STUN\n")

	c, err := stun.NewClient(conn)
	failOnErr(err, "failed to stun.NewClient")

	// Building binding request with random transaction id.
	message := stun.MustBuild(stun.TransactionID, stun.BindingRequest)
	// Sending request to STUN server, waiting for response message.
	err = c.Do(message, func(res stun.Event) {
		if res.Error != nil {
			panic(res.Error)
		}
		// Decoding XOR-MAPPED-ADDRESS attribute from message.
		var xorAddr stun.XORMappedAddress
		if err := xorAddr.GetFrom(res.Message); err != nil {
			panic(err)
		}
		spew.Dump(res)
		spew.Dump(xorAddr)
		fmt.Printf("Found address: %s\n", xorAddr.String())

		conn.Close()
		chatLoop(laddr)
	})
	failOnErr(err, "failed stun Do")
}
