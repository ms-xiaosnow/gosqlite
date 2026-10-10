#!/usr/bin/env bash
set -uo pipefail
root=/workspace/scratch/5e06a74e5f65/durable-restart
cd "$root/native-check-independent-replay-v2"
export GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOFLAGS=-buildvcs=false GOCACHE="$root/go-cache" GOPATH="$root/go-path" CGO_ENABLED=0 SQLITE_ORACLE="$root/toolchains/sqlite-build-3540000/sqlite3"
go="$root/toolchains/go1.27.2/bin/go"; ev="$root/native-check-independent-evidence"
run(){ name="$1";shift;printf '%q ' "$@" > "$ev/$name.command";printf '\n' >> "$ev/$name.command";"$@" > "$ev/$name.log" 2>&1;code=$?;echo "$code" > "$ev/$name.exit";cat "$ev/$name.log";if [[ $code != 0 ]];then exit "$code";fi; }
run guard-v2 "$go" run ./internal/testscope
run normal-v2 "$go" test ./sqlite ./cmd/gosqlite -run '^(TestNativeCheck|TestCLINativeCheck|TestIndependentCheck)' -count=1 -v
run race-guard-v2 "$go" run ./internal/testscope
run race-v2 env CGO_ENABLED=1 "$go" test -race ./sqlite ./cmd/gosqlite -run '^(TestNativeCheck|TestCLINativeCheck|TestIndependentCheck)' -count=1
run build-v2 "$go" build -trimpath -o "$ev/gosqlite-v2" ./cmd/gosqlite
run actual-cli-v2 "$go" run ./examples/native-check "$ev/gosqlite-v2"
