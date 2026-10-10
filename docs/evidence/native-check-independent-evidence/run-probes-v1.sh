#!/usr/bin/env bash
set -uo pipefail
root=/workspace/scratch/5e06a74e5f65/durable-restart
cd "$root/native-check-independent-probes-v1"
export GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOFLAGS=-buildvcs=false GOCACHE="$root/go-cache" GOPATH="$root/go-path" CGO_ENABLED=0 SQLITE_ORACLE="$root/toolchains/sqlite-build-3540000/sqlite3"
go="$root/toolchains/go1.27.2/bin/go"; ev="$root/native-check-independent-evidence"
"$go" run ./internal/testscope > "$ev/probe-guard-v1.log" 2>&1; c=$?;echo "$c" > "$ev/probe-guard-v1.exit"; cat "$ev/probe-guard-v1.log"; [[ $c == 0 ]] || exit "$c"
"$go" test ./sqlite -run '^TestIndependentCheck' -count=1 -v > "$ev/negative-probes-v1.log" 2>&1; c=$?; echo "$c" > "$ev/negative-probes-v1.exit"; cat "$ev/negative-probes-v1.log";exit "$c"
