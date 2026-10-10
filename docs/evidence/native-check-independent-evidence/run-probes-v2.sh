#!/usr/bin/env bash
set -uo pipefail
root=/workspace/scratch/5e06a74e5f65/durable-restart
cd "$root/native-check-independent-probes-v2"
export GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOFLAGS=-buildvcs=false GOCACHE="$root/go-cache" GOPATH="$root/go-path" CGO_ENABLED=0 SQLITE_ORACLE="$root/toolchains/sqlite-build-3540000/sqlite3"
go="$root/toolchains/go1.27.2/bin/go";ev="$root/native-check-independent-evidence"
run(){ name="$1";shift;printf '%q ' "$@" > "$ev/$name.command";printf '\n' >> "$ev/$name.command";"$@" > "$ev/$name.log" 2>&1;code=$?;echo "$code" > "$ev/$name.exit";cat "$ev/$name.log";if [[ $code != 0 ]];then exit "$code";fi; }
run probes-guard-v2 "$go" run ./internal/testscope
run probes-normal-v2 "$go" test ./sqlite ./driver -run '^TestIndependent' -count=1 -v
run probes-race-guard-v2 "$go" run ./internal/testscope
run probes-race-v2 env CGO_ENABLED=1 "$go" test -race ./sqlite ./driver -run '^TestIndependent' -count=1 -v
run probes-vet-v2 "$go" vet ./sqlite ./driver ./cmd/gosqlite ./internal/btree
