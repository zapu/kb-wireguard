package main

import (
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/chzyer/readline"
)

func failOnErr(err error, msg string) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", msg, err)
	}
}

func main() {
	conn, err := net.Dial("udp", "100.0.0.255:7777")
	failOnErr(err, "UDP dial")

	addr := &net.UDPAddr{
		Port: 7777,
	}
	listener, err := net.ListenUDP("udp", addr)
	failOnErr(err, "ListenUDP")

	_ = conn

	rl, err := readline.New("> ")
	failOnErr(err, "readline.New")
	defer rl.Close()

	go func() {
		buffer := make([]byte, 32*1024)
		for {
			n, from, err := listener.ReadFromUDP(buffer)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed ReadFromUDP: %s\n", err)
				continue
			}
			str := strings.TrimSpace(string(buffer[:n]))
			fmt.Fprintf(rl.Stdout(), "[%s]: %s\n", from.IP.String(), str)
		}
	}()

	for {
		line, err := rl.Readline()
		if err != nil {
			break
		}
		fmt.Printf("%s\n", line)
		_, err = conn.Write([]byte(line))
		if err != nil {
			fmt.Fprintf(rl.Stderr(), "Failed to send: %s\n", err)
		}
	}
}
