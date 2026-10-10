#!/usr/bin/env bash
set -euo pipefail
cd /workspace/scratch/5e06a74e5f65/durable-restart/boolean-rank-independent-copy-v1
export PATH="$PWD/../toolchains/go1.27.2/bin:$PATH" GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOCACHE="$PWD/../go-cache" GOPATH="$PWD/../go-path" CGO_ENABLED=0 GOFLAGS=-buildvcs=false SQLITE_ORACLE="$PWD/../toolchains/sqlite-build-3540000/sqlite3"
date -u +%FT%TZ
go version
"$SQLITE_ORACLE" :memory: 'SELECT sqlite_version(),sqlite_source_id();'
go run ./internal/testscope
go test ./sqlite ./internal/sql -run '^(TestIndependentRank|TestBoolean|TestSparseReviewMemoryLazyNullAndSupportBudgets$|TestSparseReviewMemoryExternalDefaultsAndAliases$|TestANNReviewEmptyIndexAndAllFilteredSQL$)' -count=1 -timeout=10m -v
go run ./internal/testscope
go test -tags oracle ./internal/testkit -run '^TestBooleanRankFilterOracle$' -count=1 -timeout=10m -v
go run ./internal/testscope
CGO_ENABLED=1 go test -race ./sqlite -run '^TestIndependentRank' -count=1 -timeout=10m -v
date -u +%FT%TZ
echo 'INDEPENDENT SELECTED CHECKS PASS'
