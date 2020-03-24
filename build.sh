#!/bin/bash
set -euo pipefail
go build ./cmd/run-dev && go build ./cmd/kb-wireguard

