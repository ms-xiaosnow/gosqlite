#!/bin/bash
set -euo pipefail
export PATH="$PWD/../toolchains/go1.27.2/bin:$PATH" GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOCACHE="$PWD/../go-cache" GOPATH="$PWD/../go-path" CGO_ENABLED=0 GOFLAGS=-buildvcs=false GOMAXPROCS=2
out="$PWD/docs/evidence/journal-segments-independent"
for role in baseline work; do
 (
 cd "../journal-segments-independent-${role}-v1"
 go run ./internal/testscope
 go test ./internal/journal -run '^TestIndependentSingleSegmentCompatibilityDigest$' -count=1 -v
 ) > "$out/single-compatibility-${role}.txt" 2>&1
 grep 'COMPATIBILITY samples=' "$out/single-compatibility-${role}.txt" | sed 's/^.*COMPATIBILITY/COMPATIBILITY/' > "$out/single-compatibility-${role}-digest.txt"
 done
cmp "$out/single-compatibility-baseline-digest.txt" "$out/single-compatibility-work-digest.txt"
cat "$out/single-compatibility-work-digest.txt"
echo 'BASELINE/FINAL SINGLE-SEGMENT PLAN DIGESTS IDENTICAL'
