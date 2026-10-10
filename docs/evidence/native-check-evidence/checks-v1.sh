#!/usr/bin/env bash
set -uo pipefail
root=/workspace/scratch/5e06a74e5f65/durable-restart
cd "$root/native-check-freeze-v1"
export GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOFLAGS=-buildvcs=false GOCACHE="$root/go-cache" GOPATH="$root/go-path" CGO_ENABLED=0 SQLITE_ORACLE="$root/toolchains/sqlite-build-3540000/sqlite3"
go="$root/toolchains/go1.27.2/bin/go";ev="$root/native-check-evidence"
run(){ name="$1";shift;printf '%q ' "$@" > "$ev/$name.command";printf '\n' >> "$ev/$name.command";"$@" > "$ev/$name.txt" 2>&1;code=$?;echo "$code" > "$ev/$name.exit";cat "$ev/$name.txt";if [[ $code != 0 ]];then exit "$code";fi; }
run frozen-toolchain-v1 "$go" version
run frozen-oracle-v1 "$SQLITE_ORACLE" :memory: 'SELECT sqlite_version(),sqlite_source_id();'
run frozen-scope-v1 "$go" run ./internal/testscope
run frozen-normal-v1 "$go" test ./sqlite ./cmd/gosqlite -run '^(TestNativeCheck|TestCLINativeCheck|TestCLIGramsQueryAndCheck|TestSQLWriteAndQuery|TestVersion)$|^(TestNativeCheck|TestCLINativeCheck)' -count=1 -v
run frozen-race-scope-v1 "$go" run ./internal/testscope
run frozen-race-v1 env CGO_ENABLED=1 "$go" test -race ./sqlite ./cmd/gosqlite -run '^(TestNativeCheck|TestCLINativeCheck|TestCLIGramsQueryAndCheck|TestSQLWriteAndQuery|TestVersion)$|^(TestNativeCheck|TestCLINativeCheck)' -count=1
run frozen-vet-v1 "$go" vet ./sqlite ./cmd/gosqlite ./examples/native-check
run frozen-build-v1 "$go" build -trimpath -o "$ev/gosqlite-v1" ./cmd/gosqlite
run frozen-cli-proof-v1 "$go" run ./examples/native-check "$ev/gosqlite-v1"
run frozen-cgo-deps-v1 "$go" list -deps -f '{{if .CgoFiles}}{{.ImportPath}}{{end}}' ./cmd/gosqlite
run frozen-module-v1 "$go" mod verify
