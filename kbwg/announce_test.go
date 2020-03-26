package kbwg

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParse(t *testing.T) {
	msg := "ANNOUNCE 192.168.0.164:51820 LhdznlMunOticjwvG+WdHk2f9aYGvXugcrDhG2MJeBA="
	ann, ok := ParseAnnounceMsg(msg)
	require.True(t, ok)
	require.Equal(t, "192.168.0.164:51820", ann.Endpoint)
	require.Equal(t, "LhdznlMunOticjwvG+WdHk2f9aYGvXugcrDhG2MJeBA=", ann.PublicKey)
}
