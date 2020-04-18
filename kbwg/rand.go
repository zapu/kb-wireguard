package kbwg

import (
	"crypto/rand"
	"fmt"
)

func RandBytes(length int) []byte {
	var n int
	var err error
	buf := make([]byte, length)
	if n, err = rand.Read(buf); err != nil {
		panic(err)
	}
	// rand.Read uses io.ReadFull internally, so this check should never fail.
	if n != length {
		panic(fmt.Errorf("RandBytes got too few bytes, %d < %d", n, length))
	}
	return buf
}
