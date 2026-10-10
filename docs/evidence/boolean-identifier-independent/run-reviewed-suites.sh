#!/bin/sh
set -u
export GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOFLAGS=-buildvcs=false
export GOPATH="$PWD/../go-path" GOCACHE="$PWD/../go-cache" SQLITE_ORACLE="$PWD/../toolchains/sqlite-build-3540000/sqlite3" CGO_ENABLED=0
GO=../toolchains/go1.27.2/bin/go
E=../gosqlite/docs/evidence/boolean-identifier-independent
run() { label=$1; shift; { printf '%s\n' "$*"; "$GO" run ./internal/testscope && "$@"; } > "$E/$label.txt" 2>&1; result=$?; echo "$result" > "$E/$label.exit"; echo "$label: $result"; }
run unchanged-identifier-matrix "$GO" test -tags oracle ./internal/testkit -run '^TestBooleanIdentifierOracle(Matrix|DML)$' -count=1 -v
run unchanged-provenance-matrix "$GO" test -tags oracle ./internal/testkit -run '^TestBooleanAliasOnOracleProvenance$' -count=1 -v
run owned-native "$GO" test ./internal/sql ./sqlite ./driver -run '^Test(BooleanIdentifier|BooleanAliasOn|DriverBooleanIdentifier|DriverBooleanAliasOn)' -count=1 -v
export CGO_ENABLED=1
run owned-race "$GO" test -race ./internal/sql ./sqlite ./driver -run '^Test(BooleanIdentifier|BooleanAliasOn|DriverBooleanIdentifier|DriverBooleanAliasOn)' -count=1
export CGO_ENABLED=0
run build "$GO" build ./...
run vet "$GO" vet ./internal/sql ./sqlite ./driver
