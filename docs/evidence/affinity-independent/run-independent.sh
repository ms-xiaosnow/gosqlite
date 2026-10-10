#!/bin/sh
set -eu
# Run from the reviewer isolated clone, not the immutable source freeze.
export GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off
export GOPATH="$PWD/../go-path" GOCACHE="$PWD/../go-cache"
export GOFLAGS=-buildvcs=false SQLITE_ORACLE="$PWD/../toolchains/sqlite-build-3540000/sqlite3"
GO=../toolchains/go1.27.2/bin/go
export CGO_ENABLED=0
"$GO" run ./internal/testscope
"$GO" test -tags oracle ./internal/testkit -run '^TestAffinityIndependentCrossValuesOracle$' -count=1 -v
"$GO" run ./internal/testscope
"$GO" test -tags oracle ./internal/testkit -run '^TestAffinity(InheritedMatrixOracle|ComparisonMatrixOracle|ComparisonWritesOracle)$' -count=1 -v
"$GO" run ./internal/testscope
"$GO" vet -tags oracle ./internal/sql ./internal/testkit ./sqlite
export CGO_ENABLED=1
"$GO" run ./internal/testscope
"$GO" test -race ./internal/sql ./sqlite -run '^TestAffinity' -count=1 -v
