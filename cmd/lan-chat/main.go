package main

import (
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/chzyer/readline"
)

func failOnErr(err error, msg string) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", msg, err)
	}
}

const Port = 7777

// Magic marks lan-chat packets so we can filter other noise that might be
// coming on the port.
const Magic = "😹"

var Conns []net.Conn

func makeConns() {
	var s byte
	for s = 1; s < 255; s++ {
		ip := net.IPv4(100, 0, 0, s)
		conn, err := net.Dial("udp", fmt.Sprintf("%s:%d", ip.String(), Port))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to dial to %s: %s\n", ip, err)
			continue
		}
		Conns = append(Conns, conn)
	}
}

func broadcast(line string, stderr io.Writer) {
	for _, conn := range Conns {
		_, _ = conn.Write(append([]byte(Magic), []byte(line)...))
	}
}

func main() {
	_, magicSize := utf8.DecodeRuneInString(Magic)

	addr := &net.UDPAddr{
		Port: Port,
	}
	listener, err := net.ListenUDP("udp", addr)
	failOnErr(err, "ListenUDP")

	makeConns()

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
			line := string(buffer[:n])
			// fmt.Fprintf(rl.Stdout(), "[%s]: %s\n", from.IP.String(), str)
			if strings.HasPrefix(line, Magic) {
				str := strings.TrimSpace(line[magicSize:])
				fmt.Fprintf(rl.Stdout(), "[%s]: %s\n", from.IP.String(), str)
			}
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
