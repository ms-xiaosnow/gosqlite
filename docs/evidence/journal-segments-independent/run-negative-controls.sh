#!/bin/bash
set -euo pipefail
export PATH="$PWD/../toolchains/go1.27.2/bin:$PATH" GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOCACHE="$PWD/../go-cache" GOPATH="$PWD/../go-path" CGO_ENABLED=0 GOFLAGS=-buildvcs=false GOMAXPROCS=2
out="$PWD/docs/evidence/journal-segments-independent"
for pair in nonce:TestIndependentSegmentGeometryAndOwnership quota:TestIndependentSegmentLimitsAndOverflow partial:TestIndependentSegmentEveryPrefixAndTail ownership:TestIndependentSegmentGeometryAndOwnership; do
 name=${pair%:*}; test=${pair#*:}
 (
 cd "../journal-segments-independent-negative-${name}-v1"
 go run ./internal/testscope
 set +e
 go test ./internal/journal -run "^${test}$" -count=1 -timeout=2m
 status=$?
 echo "EXPECTED NEGATIVE CONTROL EXIT: $status"
 test "$status" -ne 0
 ) > "$out/negative-${name}.txt" 2>&1
 grep -q -- "--- FAIL: ${test}" "$out/negative-${name}.txt"
 echo "$name: expected regression caught"
 done
echo 'ALL FOUR SOURCE-MUTATION NEGATIVE CONTROLS SENSITIZED'
