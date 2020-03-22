package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Printf("Hello from devowner %d %d\n", os.Getuid(), os.Geteuid())
}
