#!/usr/bin/env bash
set -euo pipefail
cd /workspace/scratch/5e06a74e5f65/durable-restart/boolean-rank-filter-freeze-v1
export PATH="$PWD/../toolchains/go1.27.2/bin:$PATH" GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOCACHE="$PWD/../go-cache" GOPATH="$PWD/../go-path" CGO_ENABLED=1 GOFLAGS=-buildvcs=false
cat ../boolean-rank-filter-evidence/scope-audit.md
echo 'Race instrumentation only; no physical race injection or manual tags.'
date -u +%FT%TZ
sha256sum -c ../boolean-rank-filter-evidence/frozen-all-sha256.txt >/dev/null
go run ./internal/testscope
go test -race ./sqlite ./internal/sql ./driver ./extension/vector/... -run '(Boolean|Vector|ANN|Half|Sparse|Bit|IVF|Similarity|Expression)' -count=1 -timeout=20m
sha256sum -c ../boolean-rank-filter-evidence/frozen-all-sha256.txt >/dev/null
date -u +%FT%TZ
echo 'FROZEN SELECTED RACE PASS'
