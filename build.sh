#!/bin/bash
set -euo pipefail
go build ./cmd/run-dev
go build ./cmd/kb-wireguard
go build ./cmd/lan-chat
go build ./cmd/stun-test
go build ./cmd/directconnect
