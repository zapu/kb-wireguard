package main

import (
	"fmt"
	"io"
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

const Port = 7777

func broadcast(line string, stderr io.Writer) {
	var s byte
	for s = 1; s < 255; s++ {
		ip := net.IPv4(100, 0, 0, s)
		conn, err := net.Dial("udp", fmt.Sprintf("%s:%d", ip.String(), Port))
		if err != nil {
			fmt.Fprintf(stderr, "Failed to dial to %s\n", ip)
			continue
		}
		_, err = conn.Write([]byte(line))
		if err != nil {
			continue
		}
	}
}

func main() {
	addr := &net.UDPAddr{
		Port: Port,
	}
	listener, err := net.ListenUDP("udp", addr)
	failOnErr(err, "ListenUDP")

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
		broadcast(line, rl.Stderr())
	}
}
