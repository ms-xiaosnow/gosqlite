#!/bin/bash
set -u
ROOT=/workspace/scratch/5e06a74e5f65/durable-restart
E="$ROOT/transaction-soak-independent-review-v4"
export PATH="$ROOT/toolchains/go1.27.2/bin:$PATH"
export GOCACHE="$ROOT/go-cache" GOPATH="$ROOT/go-path" GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOFLAGS=-buildvcs=false CGO_ENABLED=0
run() { name="$1"; shift; printf '\n%s ' "$(date -u +%FT%TZ)"; printf '%q ' "$@"; printf '\n'; "$@" > "$E/$name.txt" 2>&1; s=$?; echo "$s" > "$E/$name.exit"; cat "$E/$name.txt"; echo "exit=$s"; return "$s"; }
cd "$E/runtime"
run guard go run ./internal/testscope || exit
run normal go test ./benchmarks/transactions -count=1 -timeout=0 -v || exit
run race env CGO_ENABLED=1 go test -race ./benchmarks/transactions -count=1 -timeout=0 -v || exit
run vet go vet ./benchmarks/transactions || exit
run build go build -trimpath -o "$E/transactions" ./benchmarks/transactions || exit
