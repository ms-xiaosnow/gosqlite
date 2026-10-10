#!/bin/bash
set -euo pipefail
export PATH="$PWD/../toolchains/go1.27.2/bin:$PATH" GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOCACHE="$PWD/../go-cache" GOPATH="$PWD/../go-path" CGO_ENABLED=0 GOFLAGS=-buildvcs=false GOMAXPROCS=2
for target in FuzzReplayPlanBytesOnly FuzzReplaySegmentsBytesOnly FuzzIndependentSegmentStructured; do
 go run ./internal/testscope
 go test ./internal/journal -run '^$' -fuzz "^${target}$" -fuzztime=10s -parallel=2
 done
echo 'THREE BOUNDED PURE-MEMORY FUZZ TARGETS PASS'
