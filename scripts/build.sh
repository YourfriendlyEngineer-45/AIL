#!/bin/sh
set -eu
CGO_ENABLED=0 go build -o ail ./cmd/ail
