#!/usr/bin/env bash
set -uo pipefail
root=/workspace/scratch/5e06a74e5f65/durable-restart
cd "$root/native-check-freeze-v2"
export GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOFLAGS=-buildvcs=false GOCACHE="$root/go-cache" GOPATH="$root/go-path" CGO_ENABLED=0 SQLITE_ORACLE="$root/toolchains/sqlite-build-3540000/sqlite3"
go="$root/toolchains/go1.27.2/bin/go";ev="$root/native-check-evidence"
run(){ name="$1";shift;printf '%q ' "$@" > "$ev/$name.command";printf '\n' >> "$ev/$name.command";"$@" > "$ev/$name.txt" 2>&1;code=$?;echo "$code" > "$ev/$name.exit";cat "$ev/$name.txt";if [[ $code != 0 ]];then exit "$code";fi; }
run frozen-toolchain-v2 "$go" version
run frozen-oracle-v2 "$SQLITE_ORACLE" :memory: 'SELECT sqlite_version(),sqlite_source_id();'
run frozen-scope-v2 "$go" run ./internal/testscope
run frozen-normal-v2 "$go" test ./sqlite ./cmd/gosqlite -run '^(TestNativeCheck|TestCLINativeCheck|TestCLIGramsQueryAndCheck|TestSQLWriteAndQuery|TestVersion)$|^(TestNativeCheck|TestCLINativeCheck|TestIndependentCheck)' -count=1 -v
run frozen-race-scope-v2 "$go" run ./internal/testscope
run frozen-race-v2 env CGO_ENABLED=1 "$go" test -race ./sqlite ./cmd/gosqlite -run '^(TestNativeCheck|TestCLINativeCheck|TestCLIGramsQueryAndCheck|TestSQLWriteAndQuery|TestVersion)$|^(TestNativeCheck|TestCLINativeCheck|TestIndependentCheck)' -count=1
run frozen-vet-v2 "$go" vet ./sqlite ./cmd/gosqlite ./examples/native-check
run frozen-build-v2 "$go" build -trimpath -o "$ev/gosqlite-v2" ./cmd/gosqlite
run frozen-cli-proof-v2 "$go" run ./examples/native-check "$ev/gosqlite-v2"
run frozen-cgo-deps-v2 "$go" list -deps -f '{{if .CgoFiles}}{{.ImportPath}}{{end}}' ./cmd/gosqlite
run frozen-module-v2 "$go" mod verify
run frozen-btree-vet-v2 "$go" vet ./internal/btree
