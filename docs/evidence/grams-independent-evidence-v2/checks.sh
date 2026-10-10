#!/bin/bash
set -euo pipefail
b=/workspace/scratch/5e06a74e5f65/durable-restart
e="$b/grams-independent-evidence-v2"
cd "$b/grams-independent-review-v2"
export PATH="$b/toolchains/go1.27.2/bin:$PATH" GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOCACHE="$b/go-cache" GOPATH="$b/go-path" CGO_ENABLED=0 GOFLAGS=-buildvcs=false GOMAXPROCS=2
trap 's=$?; echo "$s" > "$e/checks.exit"' EXIT
date -u +%FT%TZ
go version
go run ./internal/testscope
go test ./sqlite -run '^TestGrams' -count=1 -v -timeout=12m
go test ./benchmarks/grams -count=1 -v -timeout=12m
go vet ./benchmarks/grams
go build -trimpath -o "$e/grams-independent" ./benchmarks/grams
go version -m "$e/grams-independent" > "$e/build-info.txt"
sha256sum "$e/grams-independent" > "$e/binary-sha256.txt"
go list -deps -f '{{if .CgoFiles}}{{.ImportPath}}:{{.CgoFiles}}{{end}}' ./benchmarks/grams | sed '/^$/d' > "$e/cgo-dependencies.txt"
test ! -s "$e/cgo-dependencies.txt"
go run ./internal/testscope
CGO_ENABLED=1 go test -race ./sqlite -run '^TestGrams' -count=1 -v -timeout=20m
CGO_ENABLED=1 go test -race ./benchmarks/grams -count=1 -v -timeout=15m
date -u +%FT%TZ
