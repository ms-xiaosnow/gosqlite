#!/usr/bin/env bash
set -uo pipefail
root=/workspace/scratch/5e06a74e5f65/durable-restart
cd "$root/native-check-independent-replay-v1"
export GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOFLAGS=-buildvcs=false GOCACHE="$root/go-cache" GOPATH="$root/go-path" CGO_ENABLED=0 SQLITE_ORACLE="$root/toolchains/sqlite-build-3540000/sqlite3"
go="$root/toolchains/go1.27.2/bin/go"; ev="$root/native-check-independent-evidence"
run(){ name="$1";shift;printf '%q ' "$@" > "$ev/$name.command";printf '\n' >> "$ev/$name.command";"$@" > "$ev/$name.log" 2>&1;code=$?;echo "$code" > "$ev/$name.exit";cat "$ev/$name.log";if [[ $code != 0 ]];then exit "$code";fi; }
run guard-v1 "$go" run ./internal/testscope
run normal-v1 "$go" test ./sqlite ./cmd/gosqlite -run '^(TestNativeCheck|TestCLINativeCheck)' -count=1 -v
run race-guard-v1 "$go" run ./internal/testscope
run race-v1 env CGO_ENABLED=1 "$go" test -race ./sqlite ./cmd/gosqlite -run '^(TestNativeCheck|TestCLINativeCheck)' -count=1
run build-v1 "$go" build -trimpath -o "$ev/gosqlite-v1" ./cmd/gosqlite
run actual-cli-v1 "$go" run ./examples/native-check "$ev/gosqlite-v1"
